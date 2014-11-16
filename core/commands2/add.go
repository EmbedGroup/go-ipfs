package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	pinning "github.com/jbenet/go-ipfs/pin"
	u "github.com/jbenet/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

type AddOutput struct {
	Objects []*Object
	Names   []string
}

var addCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add an object to ipfs.",
		ShortDescription: `
Adds contents of <path> to ipfs. Use -r to add directories.
Note that directories are added recursively, to form the ipfs
MerkleDAG. A smarter partial add with a staging area (like git)
remains to be implemented.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Must be specified when adding directories"),
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path to a file to be added to IPFS"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		added := &AddOutput{}
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		_, _, err = req.Option("r").Bool()
		if err != nil {
			return nil, err
		}

		// returns the last one
		addDagnode := func(name string, dn *dag.Node) error {
			o, err := getOutput(dn)
			if err != nil {
				return err
			}

			added.Objects = append(added.Objects, o)
			added.Names = append(added.Names, name)
			return nil
		}

		addFile := func(file cmds.File) (*dag.Node, error) {
			dns, err := add(n, []io.Reader{file})
			if err != nil {
				return nil, err
			}

			log.Infof("adding file: %s", file.FileName())
			if err := addDagnode(file.FileName(), dns[len(dns)-1]); err != nil {
				return nil, err
			}
			return dns[len(dns)-1], nil // last dag node is the file.
		}

		// TODO: handle directories

		file, err := req.Files().NextFile()
		for file != nil {
			_, err := addFile(file)
			if err != nil {
				return nil, err
			}

			file, err = req.Files().NextFile()
		}
		if err != nil && err != io.EOF {
			return nil, err
		}

		return added, nil

		// readers, err := internal.CastToReaders(req.Arguments())
		// if err != nil {
		// 	return nil, err
		// }
		//
		// dagnodes, err := add(n, readers)
		// if err != nil {
		// 	return nil, err
		// }
		//
		// // TODO: include fs paths in output (will need a way to specify paths in underlying filearg system)
		// added := make([]*Object, 0, len(req.Arguments()))
		// for _, dagnode := range dagnodes {
		// 	object, err := getOutput(dagnode)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		//
		// 	added = append(added, object)
		// }
		//
		// return &AddOutput{added}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			val, ok := res.Output().(*AddOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			var buf bytes.Buffer
			for i, obj := range val.Objects {
				buf.Write([]byte(fmt.Sprintf("added %s %s\n", obj.Hash, val.Names[i])))
			}
			return buf.Bytes(), nil
		},
	},
	Type: &AddOutput{},
}

func add(n *core.IpfsNode, readers []io.Reader) ([]*dag.Node, error) {
	mp, ok := n.Pinning.(pinning.ManualPinner)
	if !ok {
		return nil, errors.New("invalid pinner type! expected manual pinner")
	}

	dagnodes := make([]*dag.Node, 0)

	// TODO: allow adding directories (will need support for multiple files in filearg system)

	for _, reader := range readers {
		node, err := importer.BuildDagFromReader(reader, n.DAG, mp, chunk.DefaultSplitter)
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, node)
	}

	return dagnodes, nil
}

func addNode(n *core.IpfsNode, node *dag.Node) error {
	err := n.DAG.AddRecursive(node) // add the file to the graph + local storage
	if err != nil {
		return err
	}

	err = n.Pinning.Pin(node, true) // ensure we keep it
	if err != nil {
		return err
	}

	return nil
}

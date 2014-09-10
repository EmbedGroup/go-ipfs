// FUSE directory tree, for servers that wish to use it with the service loop.

package fs

import (
	"os"
	pathpkg "path"
	"strings"
)

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
)

// A Tree implements a basic read-only directory tree for FUSE.
// The Nodes contained in it may still be writable.
type Tree struct {
	tree
}

func (t *Tree) Root() (Node, fuse.Error) {
	return &t.tree, nil
}

// Add adds the path to the tree, resolving to the given node.
// If path or a prefix of path has already been added to the tree,
// Add panics.
//
// Add is only safe to call before starting to serve requests.
func (t *Tree) Add(path string, node Node) {
	path = pathpkg.Clean("/" + path)[1:]
	elems := strings.Split(path, "/")
	dir := Node(&t.tree)
	for i, elem := range elems {
		dt, ok := dir.(*tree)
		if !ok {
			panic("fuse: Tree.Add for " + strings.Join(elems[:i], "/") + " and " + path)
		}
		n := dt.lookup(elem)
		if n != nil {
			if i+1 == len(elems) {
				panic("fuse: Tree.Add for " + path + " conflicts with " + elem)
			}
			dir = n
		} else {
			if i+1 == len(elems) {
				dt.add(elem, node)
			} else {
				dir = &tree{}
				dt.add(elem, dir)
			}
		}
	}
}

type treeDir struct {
	name string
	node Node
}

type tree struct {
	dir []treeDir
}

func (t *tree) lookup(name string) Node {
	for _, d := range t.dir {
		if d.name == name {
			return d.node
		}
	}
	return nil
}

func (t *tree) add(name string, n Node) {
	t.dir = append(t.dir, treeDir{name, n})
}

func (t *tree) Attr() fuse.Attr {
	return fuse.Attr{Mode: os.ModeDir | 0555}
}

func (t *tree) Lookup(name string, intr Intr) (Node, fuse.Error) {
	n := t.lookup(name)
	if n != nil {
		return n, nil
	}
	return nil, fuse.ENOENT
}

func (t *tree) ReadDir(intr Intr) ([]fuse.Dirent, fuse.Error) {
	var out []fuse.Dirent
	for _, d := range t.dir {
		out = append(out, fuse.Dirent{Name: d.name})
	}
	return out, nil
}

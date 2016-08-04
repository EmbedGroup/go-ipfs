package io

import (
	"context"
	"fmt"
	"os"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	hamt "github.com/ipfs/go-ipfs/unixfs/hamt"

	node "gx/ipfs/QmYDscK7dmdo2GZ9aumS8s5auUUAH5mR1jvj5pYhWusfK7/go-ipld-node"
)

// ShardSplitThreshold specifies how large of an unsharded directory
// the Directory code will generate. Adding entries over this value will
// result in the node being restructured into a sharded object.
var ShardSplitThreshold = 1000

// DefaultShardWidth is the default value used for hamt sharding width.
var DefaultShardWidth = 256

type Directory struct {
	dserv   mdag.DAGService
	dirnode *mdag.ProtoNode

	shard *hamt.HamtShard
}

// NewDirectory returns a Directory. It needs a DAGService to add the Children
func NewDirectory(dserv mdag.DAGService) *Directory {
	db := new(Directory)
	db.dserv = dserv
	db.dirnode = format.EmptyDirNode()
	return db
}

func NewDirectoryFromNode(dserv mdag.DAGService, nd node.Node) (*Directory, error) {
	pbnd, ok := nd.(*mdag.ProtoNode)
	if !ok {
		return nil, mdag.ErrNotProtobuf
	}

	pbd, err := format.FromBytes(pbnd.Data())
	if err != nil {
		return nil, err
	}

	switch pbd.GetType() {
	case format.TDirectory:
		return &Directory{
			dserv:   dserv,
			dirnode: pbnd.Copy().(*mdag.ProtoNode),
		}, nil
	case format.THAMTShard:
		shard, err := hamt.NewHamtFromDag(dserv, nd)
		if err != nil {
			return nil, err
		}

		return &Directory{
			dserv: dserv,
			shard: shard,
		}, nil
	default:
		return nil, fmt.Errorf("merkledag node was not a directory or shard")
	}
}

// AddChild adds a (name, key)-pair to the root node.
func (d *Directory) AddChild(ctx context.Context, name string, nd node.Node) error {
	if d.shard == nil {
		if len(d.dirnode.Links()) < ShardSplitThreshold {
			_ = d.dirnode.RemoveNodeLink(name)
			return d.dirnode.AddNodeLinkClean(name, nd)
		}

		err := d.switchToSharding(ctx)
		if err != nil {
			return err
		}
	}

	return d.shard.Set(ctx, name, nd)
}

func (d *Directory) switchToSharding(ctx context.Context) error {
	d.shard = hamt.NewHamtShard(d.dserv, DefaultShardWidth)
	for _, lnk := range d.dirnode.Links() {
		cnd, err := d.dserv.Get(ctx, lnk.Cid)
		if err != nil {
			return err
		}

		err = d.shard.Set(ctx, lnk.Name, cnd)
		if err != nil {
			return err
		}
	}

	d.dirnode = nil
	return nil
}

func (d *Directory) Links() ([]*node.Link, error) {
	if d.shard == nil {
		return d.dirnode.Links(), nil
	}

	return d.shard.EnumLinks()
}

func (d *Directory) Find(ctx context.Context, name string) (node.Node, error) {
	if d.shard == nil {
		lnk, err := d.dirnode.GetNodeLink(name)
		switch err {
		case mdag.ErrLinkNotFound:
			return nil, os.ErrNotExist
		default:
			return nil, err
		case nil:
		}

		return d.dserv.Get(ctx, lnk.Cid)
	}

	return d.shard.Find(ctx, name)
}

func (d *Directory) RemoveChild(ctx context.Context, name string) error {
	if d.shard == nil {
		return d.dirnode.RemoveNodeLink(name)
	}

	return d.shard.Remove(ctx, name)
}

// GetNode returns the root of this Directory
func (d *Directory) GetNode() (node.Node, error) {
	if d.shard == nil {
		return d.dirnode, nil
	}

	return d.shard.Node()
}

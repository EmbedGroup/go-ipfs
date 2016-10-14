package merkledag_test

import (
	"context"
	"testing"

	. "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"

	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
)

func TestRemoveLink(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*node.Link{
		{Name: "a"},
		{Name: "b"},
		{Name: "a"},
		{Name: "a"},
		{Name: "c"},
		{Name: "a"},
	})

	err := nd.RemoveNodeLink("a")
	if err != nil {
		t.Fatal(err)
	}

	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}

	// should fail
	err = nd.RemoveNodeLink("a")
	if err != ErrNotFound {
		t.Fatal("should have failed to remove link")
	}

	// ensure nothing else got touched
	if len(nd.Links()) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links()[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links()[1].Name != "c" {
		t.Fatal("link order wrong")
	}
}

func TestFindLink(t *testing.T) {
	ds := mdtest.Mock()
	k, err := ds.Add(new(ProtoNode))
	if err != nil {
		t.Fatal(err)
	}

	nd := &ProtoNode{}
	nd.SetLinks([]*node.Link{
		{Name: "a", Cid: k},
		{Name: "c", Cid: k},
		{Name: "b", Cid: k},
	})

	_, err = ds.Add(nd)
	if err != nil {
		t.Fatal(err)
	}

	lnk, err := nd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if lnk.Name != "b" {
		t.Fatal("got wrong link back")
	}

	_, err = nd.GetNodeLink("f")
	if err != ErrLinkNotFound {
		t.Fatal("shouldnt have found link")
	}

	_, err = nd.GetLinkedNode(context.Background(), ds, "b")
	if err != nil {
		t.Fatal(err)
	}

	outnd, err := nd.UpdateNodeLink("b", nd)
	if err != nil {
		t.Fatal(err)
	}

	olnk, err := outnd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if olnk.Cid.String() == k.String() {
		t.Fatal("new link should have different hash")
	}
}

func TestNodeCopy(t *testing.T) {
	nd := &ProtoNode{}
	nd.SetLinks([]*node.Link{
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	})

	nd.SetData([]byte("testing"))

	ond := nd.Copy()
	ond.SetData(nil)

	if nd.Data() == nil {
		t.Fatal("should be different objects")
	}
}

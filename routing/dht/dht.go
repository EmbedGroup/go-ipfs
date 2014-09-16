package dht

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	// Array of routing tables for differently distanced nodes
	// NOTE: (currently, only a single table is used)
	routingTables []*kb.RoutingTable

	// the network interface. service
	network inet.Network
	sender  inet.Sender

	// Local peer (yourself)
	self *peer.Peer

	// Other peers
	peerstore peer.Peerstore

	// Local data
	datastore ds.Datastore
	dslock    sync.Mutex

	providers *ProviderManager

	// Signal to shutdown dht
	shutdown chan struct{}

	// When this peer started up
	birth time.Time

	//lock to make diagnostics work better
	diaglock sync.Mutex
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(p *peer.Peer, ps peer.Peerstore, net inet.Network, sender inet.Sender, dstore ds.Datastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.network = net
	dht.sender = sender
	dht.datastore = dstore
	dht.self = p
	dht.peerstore = ps

	dht.providers = NewProviderManager(p.ID)
	dht.shutdown = make(chan struct{})

	dht.routingTables = make([]*kb.RoutingTable, 3)
	dht.routingTables[0] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*30)
	dht.routingTables[1] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*100)
	dht.routingTables[2] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Hour)
	dht.birth = time.Now()
	return dht
}

// Start up background goroutines needed by the DHT
func (dht *IpfsDHT) Start() {
	panic("the service is already started. rmv this method")
}

// Connect to a new peer at the given address, ping and add to the routing table
func (dht *IpfsDHT) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	maddrstr, _ := addr.String()
	u.DOut("Connect to new peer: %s\n", maddrstr)

	// TODO(jbenet,whyrusleeping)
	//
	// Connect should take in a Peer (with ID). In a sense, we shouldn't be
	// allowing connections to random multiaddrs without knowing who we're
	// speaking to (i.e. peer.ID). In terms of moving around simple addresses
	// -- instead of an (ID, Addr) pair -- we can use:
	//
	//   /ip4/10.20.30.40/tcp/1234/ipfs/Qxhxxchxzcncxnzcnxzcxzm
	//
	npeer := &peer.Peer{}
	npeer.AddAddress(addr)
	err := dht.network.DialPeer(npeer)
	if err != nil {
		return nil, err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(npeer, time.Second*2)
	if err != nil {
		return nil, fmt.Errorf("failed to ping newly connected peer: %s\n", err)
	}

	dht.Update(npeer)

	return npeer, nil
}

// HandleMessage implements the inet.Handler interface.
func (dht *IpfsDHT) HandleMessage(ctx context.Context, mes msg.NetMessage) (msg.NetMessage, error) {

	mData := mes.Data()
	if mData == nil {
		return nil, errors.New("message did not include Data")
	}

	mPeer := mes.Peer()
	if mPeer == nil {
		return nil, errors.New("message did not include a Peer")
	}

	// deserialize msg
	pmes := new(Message)
	err := proto.Unmarshal(mData, pmes)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode protobuf message: %v\n", err)
	}

	// update the peer (on valid msgs only)
	dht.Update(mPeer)

	// Print out diagnostic
	u.DOut("[peer: %s]\nGot message type: '%s' [from = %s]\n",
		dht.self.ID.Pretty(),
		Message_MessageType_name[int32(pmes.GetType())], mPeer.ID.Pretty())

	// get handler for this msg type.
	var resp *Message
	handler := dht.handlerForMsgType(pmes.GetType())
	if handler == nil {
		return nil, errors.New("Recieved invalid message type")
	}

	// dispatch handler.
	rpmes, err := handler(mPeer, pmes)
	if err != nil {
		return nil, err
	}

	// serialize response msg
	rmes, err := msg.FromObject(mPeer, rpmes)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode protobuf message: %v\n", err)
	}

	return rmes, nil
}

// sendRequest sends out a request using dht.sender, but also makes sure to
// measure the RTT for latency measurements.
func (dht *IpfsDHT) sendRequest(ctx context.Context, p *peer.Peer, pmes *Message) (*Message, error) {

	mes, err := msg.FromObject(p, pmes)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	rmes, err := dht.sender.SendRequest(ctx, mes)
	if err != nil {
		return nil, err
	}

	rtt := time.Since(start)
	rmes.Peer().SetLatency(rtt)

	rpmes := new(Message)
	if err := proto.Unmarshal(rmes.Data(), rpmes); err != nil {
		return nil, err
	}

	return rpmes, nil
}

func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p *peer.Peer,
	key u.Key, level int) ([]byte, []*peer.Peer, error) {

	pmes, err := dht.getValueSingle(ctx, p, key, level)
	if err != nil {
		return nil, nil, err
	}

	if value := pmes.GetValue(); value != nil {
		// Success! We were given the value
		return value, nil, nil
	}

	// TODO decide on providers. This probably shouldn't be happening.
	// if prv := pmes.GetProviderPeers(); prv != nil && len(prv) > 0 {
	// 	val, err := dht.getFromPeerList(key, timeout,, level)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	return val, nil, nil
	// }

	// Perhaps we were given closer peers
	var peers []*peer.Peer
	for _, pb := range pmes.GetCloserPeers() {
		if peer.ID(pb.GetId()).Equal(dht.self.ID) {
			continue
		}

		addr, err := ma.NewMultiaddr(pb.GetAddr())
		if err != nil {
			u.PErr("%v\n", err.Error())
			continue
		}

		// check if we already have this peer.
		pr, _ := dht.peerstore.Get(peer.ID(pb.GetId()))
		if pr == nil {
			pr = &peer.Peer{ID: peer.ID(pb.GetId())}
			dht.peerstore.Put(pr)
		}
		pr.AddAddress(addr) // idempotent
		peers = append(peers, pr)
	}

	if len(peers) > 0 {
		return nil, peers, nil
	}

	return nil, nil, errors.New("NotFound. did not get value or closer peers.")
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(ctx context.Context, p *peer.Peer,
	key u.Key, level int) (*Message, error) {

	typ := Message_GET_VALUE
	skey := string(key)
	pmes := &Message{Type: &typ, Key: &skey}
	pmes.SetClusterLevel(int32(level))

	return dht.sendRequest(ctx, p, pmes)
}

// TODO: Im not certain on this implementation, we get a list of peers/providers
// from someone what do we do with it? Connect to each of them? randomly pick
// one to get the value from? Or just connect to one at a time until we get a
// successful connection and request the value from it?
func (dht *IpfsDHT) getFromPeerList(ctx context.Context, key u.Key,
	peerlist []*Message_Peer, level int) ([]byte, error) {

	for _, pinfo := range peerlist {
		p, err := dht.peerFromInfo(pinfo)
		if err != nil {
			u.DErr("getFromPeers error: %s\n", err)
			continue
		}

		pmes, err := dht.getValueSingle(ctx, p, key, level)
		if err != nil {
			u.DErr("getFromPeers error: %s\n", err)
			continue
		}

		if value := pmes.GetValue(); value != nil {
			// Success! We were given the value
			dht.providers.AddProvider(key, p)
			return value, nil
		}
	}
	return nil, u.ErrNotFound
}

func (dht *IpfsDHT) getLocal(key u.Key) ([]byte, error) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	v, err := dht.datastore.Get(ds.NewKey(string(key)))
	if err != nil {
		return nil, err
	}

	byt, ok := v.([]byte)
	if !ok {
		return byt, errors.New("value stored in datastore not []byte")
	}
	return byt, nil
}

func (dht *IpfsDHT) putLocal(key u.Key, value []byte) error {
	return dht.datastore.Put(ds.NewKey(string(key)), value)
}

// Update signals to all routingTables to Update their last-seen status
// on the given peer.
func (dht *IpfsDHT) Update(p *peer.Peer) {
	removedCount := 0
	for _, route := range dht.routingTables {
		removed := route.Update(p)
		// Only close the connection if no tables refer to this peer
		if removed != nil {
			removedCount++
		}
	}

	// Only close the connection if no tables refer to this peer
	// if removedCount == len(dht.routingTables) {
	// 	dht.network.ClosePeer(p)
	// }
	// ACTUALLY, no, let's not just close the connection. it may be connected
	// due to other things. it seems that we just need connection timeouts
	// after some deadline of inactivity.
}

// Find looks for a peer with a given ID connected to this dht and returns the peer and the table it was found in.
func (dht *IpfsDHT) Find(id peer.ID) (*peer.Peer, *kb.RoutingTable) {
	for _, table := range dht.routingTables {
		p := table.Find(id)
		if p != nil {
			return p, table
		}
	}
	return nil, nil
}

func (dht *IpfsDHT) findPeerSingle(p *peer.Peer, id peer.ID, timeout time.Duration, level int) (*Message, error) {
	pmes := Message{
		Type:  Message_FIND_NODE,
		Key:   string(id),
		ID:    swarm.GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	listenChan := dht.listener.Listen(pmes.ID, 1, time.Minute)
	t := time.Now()
	dht.netChan.Outgoing <- mes
	after := time.After(timeout)
	select {
	case <-after:
		dht.listener.Unlisten(pmes.ID)
		return nil, u.ErrTimeout
	case resp := <-listenChan:
		roundtrip := time.Since(t)
		resp.Peer.SetLatency(roundtrip)
		pmesOut := new(Message)
		err := proto.Unmarshal(resp.Data, pmesOut)
		if err != nil {
			return nil, err
		}

		return pmesOut, nil
	}
}

func (dht *IpfsDHT) printTables() {
	for _, route := range dht.routingTables {
		route.Print()
	}
}

func (dht *IpfsDHT) findProvidersSingle(p *peer.Peer, key u.Key, level int, timeout time.Duration) (*Message, error) {
	pmes := Message{
		Type:  Message_GET_PROVIDERS,
		Key:   string(key),
		ID:    swarm.GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	listenChan := dht.listener.Listen(pmes.ID, 1, time.Minute)
	dht.netChan.Outgoing <- mes
	after := time.After(timeout)
	select {
	case <-after:
		dht.listener.Unlisten(pmes.ID)
		return nil, u.ErrTimeout
	case resp := <-listenChan:
		u.DOut("FindProviders: got response.\n")
		pmesOut := new(Message)
		err := proto.Unmarshal(resp.Data, pmesOut)
		if err != nil {
			return nil, err
		}

		return pmesOut, nil
	}
}

// TODO: Could be done async
func (dht *IpfsDHT) addPeerList(key u.Key, peers []*Message_PBPeer) []*peer.Peer {
	var provArr []*peer.Peer
	for _, prov := range peers {
		// Dont add outselves to the list
		if peer.ID(prov.GetId()).Equal(dht.self.ID) {
			continue
		}
		// Dont add someone who is already on the list
		p := dht.network.GetPeer(u.Key(prov.GetId()))
		if p == nil {
			u.DOut("given provider %s was not in our network already.\n", peer.ID(prov.GetId()).Pretty())
			var err error
			p, err = dht.peerFromInfo(prov)
			if err != nil {
				u.PErr("error connecting to new peer: %s\n", err)
				continue
			}
		}
		dht.providers.AddProvider(key, p)
		provArr = append(provArr, p)
	}
	return provArr
}

// nearestPeerToQuery returns the routing tables closest peers.
func (dht *IpfsDHT) nearestPeerToQuery(pmes *Message) *peer.Peer {
	level := pmes.GetClusterLevel()
	cluster := dht.routingTables[level]

	key := u.Key(pmes.GetKey())
	closer := cluster.NearestPeer(kb.ConvertKey(key))
	return closer
}

// betterPeerToQuery returns nearestPeerToQuery, but iff closer than self.
func (dht *IpfsDHT) betterPeerToQuery(pmes *Message) *peer.Peer {
	closer := dht.nearestPeerToQuery(pmes)

	// no node? nil
	if closer == nil {
		return nil
	}

	// == to self? nil
	if closer.ID.Equal(dht.self.ID) {
		u.DOut("Attempted to return self! this shouldnt happen...\n")
		return nil
	}

	// self is closer? nil
	if kb.Closer(dht.self.ID, closer.ID, key) {
		return nil
	}

	// ok seems like a closer node.
	return closer
}

func (dht *IpfsDHT) peerFromInfo(pbp *Message_Peer) (*peer.Peer, error) {

	id := peer.ID(pbp.GetId())
	p, _ := dht.peerstore.Get(id)
	if p == nil {
		p, _ = dht.Find(id)
		if p != nil {
			panic("somehow peer not getting into peerstore")
		}
	}

	if p == nil {
		maddr, err := ma.NewMultiaddr(pbp.GetAddr())
		if err != nil {
			return nil, err
		}

		// create new Peer
		p := &peer.Peer{ID: id}
		p.AddAddress(maddr)
		dht.peerstore.Put(pr)
	}

	// dial connection
	err = dht.network.Dial(p)
	return p, err
}

func (dht *IpfsDHT) loadProvidableKeys() error {
	kl, err := dht.datastore.KeyList()
	if err != nil {
		return err
	}
	for _, k := range kl {
		dht.providers.AddProvider(u.Key(k.Bytes()), dht.self)
	}
	return nil
}

// Builds up list of peers by requesting random peer IDs
func (dht *IpfsDHT) Bootstrap() {
	id := make([]byte, 16)
	rand.Read(id)
	dht.FindPeer(peer.ID(id), time.Second*10)
}

// package swarm implements a connection muxer with a pair of channels
// to synchronize all network communication.
package swarm

import (
	peer "github.com/jbenet/go-ipfs/peer"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ps "github.com/jbenet/go-peerstream"
)

var log = eventlog.Logger("swarm2")

// Swarm is a connection muxer, allowing connections to other peers to
// be opened and closed, while still using the same Chan for all
// communication. The Chan sends/receives Messages, which note the
// destination or source Peer.
//
// Uses peerstream.Swarm
type Swarm struct {
	swarm *ps.Swarm
	local peer.Peer
	peers peer.Peerstore

	cg ctxgroup.ContextGroup
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(ctx context.Context, listenAddrs []ma.Multiaddr,
	local peer.Peer, peers peer.Peerstore) (*Swarm, error) {

	s := &Swarm{
		swarm: ps.NewSwarm(),
		local: local,
		peers: peers,
		cg:    ctxgroup.WithContext(ctx),
	}

	// configure Swarm
	s.cg.SetTeardown(s.teardown)
	s.swarm.SetConnHandler(s.connHandler)

	return s, s.listen(listenAddrs)
}

func (s *Swarm) teardown() error {
	return s.swarm.Close()
}

// Close stops the Swarm. See
func (s *Swarm) Close() error {
	return s.cg.Close()
}

// StreamSwarm returns the underlying peerstream.Swarm
func (s *Swarm) StreamSwarm() *ps.Swarm {
	return s.swarm
}

// SetStreamHandler assigns the handler for new streams.
// See peerstream.
func (s *Swarm) SetStreamHandler(handler StreamHandler) {
	s.swarm.SetStreamHandler(func(s *ps.Stream) {
		handler(wrapStream(s))
	})
}

// NewStreamWithPeer creates a new stream on any available connection to p
func (s *Swarm) NewStreamWithPeer(p peer.Peer) (*Stream, error) {
	st, err := s.swarm.NewStreamWithGroup(p)
	return wrapStream(st), err
}

// StreamsWithPeer returns all the live Streams to p
func (s *Swarm) StreamsWithPeer(p peer.Peer) []*Stream {
	return wrapStreams(ps.StreamsWithGroup(p, s.swarm.Streams()))
}

// ConnectionsToPeer returns all the live connections to p
func (s *Swarm) ConnectionsToPeer(p peer.Peer) []*SwarmConn {
	return wrapConns(ps.ConnsWithGroup(p, s.swarm.Conns()))
}

// Connections returns a slice of all connections.
func (s *Swarm) Connections() []*SwarmConn {
	return wrapConns(s.swarm.Conns())
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p peer.Peer) error {
	conns := s.swarm.ConnsWithGroup(p) // boom.
	for _, c := range conns {
		c.Close()
	}
	return nil
}

// GetPeerList returns a copy of the set of peers swarm is connected to.
func (s *Swarm) GetPeerList() []peer.Peer {
	conns := s.Connections()

	seen := make(map[peer.Peer]struct{})
	peers := make([]peer.Peer, 0, len(conns))
	for _, c := range conns {
		p := c.RemotePeer()
		if _, found := seen[p]; found {
			continue
		}

		peers = append(peers, p)
	}
	return peers
}

// LocalPeer returns the local peer swarm is associated to.
func (s *Swarm) LocalPeer() peer.Peer {
	return s.local
}

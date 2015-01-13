package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/p2p/net/conn"
	addrutil "github.com/jbenet/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
func (s *Swarm) Dial(ctx context.Context, p peer.ID) (*Conn, error) {

	if p == s.local {
		return nil, errors.New("Attempted connection to self!")
	}

	// check if we already have an open connection first
	cs := s.ConnectionsToPeer(p)
	for _, c := range cs {
		if c != nil { // dump out the first one we find
			return c, nil
		}
	}

	sk := s.peers.PrivKey(s.local)
	if sk == nil {
		// may be fine for sk to be nil, just log a warning.
		log.Warning("Dial not given PrivateKey, so WILL NOT SECURE conn.")
	}

	remoteAddrs := s.peers.Addresses(p)
	// make sure we can use the addresses.
	remoteAddrs = addrutil.FilterUsableAddrs(remoteAddrs)
	if len(remoteAddrs) == 0 {
		return nil, errors.New("peer has no addresses")
	}
	localAddrs := s.peers.Addresses(s.local)
	if len(localAddrs) == 0 {
		log.Debug("Dialing out with no local addresses.")
	}

	// open connection to peer
	d := &conn.Dialer{
		LocalPeer:  s.local,
		LocalAddrs: localAddrs,
		PrivateKey: sk,
	}

	// try to get a connection to any addr
	connC, err := s.dialAddrs(ctx, d, p, remoteAddrs)
	if err != nil {
		return nil, err
	}

	// ok try to setup the new connection.
	swarmC, err := dialConnSetup(ctx, s, connC)
	if err != nil {
		log.Error("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		swarmC.Close() // close the connection. didn't work out :(
		return nil, err
	}

	log.Event(ctx, "dial", p)
	return swarmC, nil
}

func (s *Swarm) dialAddrs(ctx context.Context, d *conn.Dialer, p peer.ID, remoteAddrs []ma.Multiaddr) (conn.Conn, error) {

	// try to connect to one of the peer's known addresses.
	// for simplicity, we do this sequentially.
	// A future commit will do this asynchronously.
	for _, addr := range remoteAddrs {
		connC, err := d.Dial(ctx, addr, p)
		if err != nil {
			continue
		}

		// if the connection is not to whom we thought it would be...
		if connC.RemotePeer() != p {
			log.Infof("misdial to %s through %s (got %s)", p, addr, connC.RemoteMultiaddr())
			connC.Close()
			continue
		}

		// if the connection is to ourselves...
		// this can happen TONS when Loopback addrs are advertized.
		// (this should be caught by two checks above, but let's just make sure.)
		if connC.RemotePeer() == s.local {
			log.Infof("misdial to %s through %s", p, addr)
			connC.Close()
			continue
		}

		// success! we got one!
		return connC, nil
	}
	return nil, fmt.Errorf("failed to dial %s", p)
}

// dialConnSetup is the setup logic for a connection from the dial side. it
// needs to add the Conn to the StreamSwarm, then run newConnSetup
func dialConnSetup(ctx context.Context, s *Swarm, connC conn.Conn) (*Conn, error) {

	psC, err := s.swarm.AddConn(connC)
	if err != nil {
		// connC is closed by caller if we fail.
		return nil, fmt.Errorf("failed to add conn to ps.Swarm: %s", err)
	}

	// ok try to setup the new connection. (newConnSetup will add to group)
	swarmC, err := s.newConnSetup(ctx, psC)
	if err != nil {
		log.Error("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		swarmC.Close() // we need to call this to make sure psC is Closed.
		return nil, err
	}

	return swarmC, err
}

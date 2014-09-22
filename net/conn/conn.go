package swarm

import (
	"fmt"
	"net"

	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// ChanBuffer is the size of the buffer in the Conn Chan
const ChanBuffer = 10

// 1 MB
const MaxMessageSize = 1 << 20

// Conn represents a connection to another Peer (IPFS Node).
type Conn struct {
	Peer *peer.Peer
	Addr *ma.Multiaddr
	Conn net.Conn

	Closed   chan bool
	Outgoing *msgio.Chan
	Incoming *msgio.Chan
	secIn    <-chan []byte
	secOut   chan<- []byte
}

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]*Conn

// Dial connects to a particular peer, over a given network
// Example: Dial("udp", peer)
func Dial(network string, peer *peer.Peer) (*Conn, error) {
	addr := peer.NetAddress(network)
	if addr == nil {
		return nil, fmt.Errorf("No address for network %s", network)
	}

	network, host, err := addr.DialArgs()
	if err != nil {
		return nil, err
	}

	nconn, err := net.Dial(network, host)
	if err != nil {
		return nil, err
	}

	conn := &Conn{
		Peer: peer,
		Addr: addr,
		Conn: nconn,
	}

	newConnChans(conn)
	return conn, nil
}

// Construct new channels for given Conn.
func newConnChans(c *Conn) error {
	if c.Outgoing != nil || c.Incoming != nil {
		return fmt.Errorf("Conn already initialized")
	}

	c.Outgoing = msgio.NewChan(10)
	c.Incoming = msgio.NewChan(10)
	c.Closed = make(chan bool, 1)

	go c.Outgoing.WriteTo(c.Conn)
	go c.Incoming.ReadFrom(c.Conn, MaxMessageSize)

	return nil
}

// Close closes the connection, and associated channels.
func (s *Conn) Close() error {
	u.DOut("Closing Conn.\n")
	if s.Conn == nil {
		return fmt.Errorf("Already closed") // already closed
	}

	// closing net connection
	err := s.Conn.Close()
	s.Conn = nil
	// closing channels
	s.Incoming.Close()
	s.Outgoing.Close()
	s.Closed <- true
	return err
}

package swarm

import (
	"errors"
	"fmt"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	conn "github.com/jbenet/go-ipfs/net/conn"
	handshake "github.com/jbenet/go-ipfs/net/handshake"
	msg "github.com/jbenet/go-ipfs/net/message"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
)

// Open listeners for each network the swarm should listen on
func (s *Swarm) listen() error {
	hasErr := false
	retErr := &ListenErr{
		Errors: make([]error, len(s.local.Addresses)),
	}

	// listen on every address
	for i, addr := range s.local.Addresses {
		err := s.connListen(addr)
		if err != nil {
			hasErr = true
			retErr.Errors[i] = err
			log.Error("Failed to listen on: %s - %s", addr, err)
		}
	}

	if hasErr {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) connListen(maddr ma.Multiaddr) error {
	list, err := manet.Listen(maddr)
	if err != nil {
		return err
	}

	// NOTE: this may require a lock around it later. currently, only run on setup
	s.listeners = append(s.listeners, list)

	// Accept and handle new connections on this listener until it errors
	go func() {
		for {
			nconn, err := list.Accept()
			if err != nil {
				e := fmt.Errorf("Failed to accept connection: %s - %s", maddr, err)
				s.errChan <- e

				// if cancel is nil, we're closed.
				if s.cancel == nil {
					return
				}
			} else {
				go s.handleIncomingConn(nconn)
			}
		}
	}()

	return nil
}

// Handle getting ID from this peer, handshake, and adding it into the map
func (s *Swarm) handleIncomingConn(nconn manet.Conn) {

	addr := nconn.RemoteMultiaddr()

	// Construct conn with nil peer for now, because we don't know its ID yet.
	// connSetup will figure this out, and pull out / construct the peer.
	c, err := conn.NewConn(nil, addr, nconn)
	if err != nil {
		s.errChan <- err
		return
	}

	// Setup the new connection
	err = s.connSetup(c)
	if err != nil && err != ErrAlreadyOpen {
		s.errChan <- err
		c.Close()
	}
}

// connSetup adds the passed in connection to its peerMap and starts
// the fanIn routine for that connection
func (s *Swarm) connSetup(c *conn.Conn) error {
	if c == nil {
		return errors.New("Tried to start nil connection.")
	}

	if c.Peer != nil {
		log.Debug("Starting connection: %s", c.Peer)
	} else {
		log.Debug("Starting connection: [unknown peer]")
	}

	if err := s.connSecure(c); err != nil {
		return fmt.Errorf("Conn securing error: %v", err)
	}

	log.Debug("Secured connection: %s", c.Peer)

	// add address of connection to Peer. Maybe it should happen in connSecure.
	c.Peer.AddAddress(c.Addr)

	if err := s.connVersionExchange(c); err != nil {
		return fmt.Errorf("Conn version exchange error: %v", err)
	}

	// add to conns
	s.connsLock.Lock()
	if _, ok := s.conns[c.Peer.Key()]; ok {
		log.Debug("Conn already open!")
		s.connsLock.Unlock()
		return ErrAlreadyOpen
	}
	s.conns[c.Peer.Key()] = c
	log.Debug("Added conn to map!")
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(c)
	return nil
}

// connSecure setups a secure remote connection.
func (s *Swarm) connSecure(c *conn.Conn) error {

	sp, err := spipe.NewSecurePipe(s.ctx, 10, s.local, s.peers)
	if err != nil {
		return err
	}

	err = sp.Wrap(s.ctx, spipe.Duplex{
		In:  c.Incoming.MsgChan,
		Out: c.Outgoing.MsgChan,
	})
	if err != nil {
		return err
	}

	if c.Peer == nil {
		c.Peer = sp.RemotePeer()

	} else if c.Peer != sp.RemotePeer() {
		panic("peers not being constructed correctly.")
	}

	c.Secure = sp
	return nil
}

// connVersionExchange exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func (s *Swarm) connVersionExchange(remote *conn.Conn) error {
	var remoteVersion, myVersion *handshake.SemVer
	myVersion = handshake.Current()

	// BUG(cryptix): do we need to use a NetMessage here?
	myVersionMsg, err := msg.FromObject(s.local, myVersion)
	if err != nil {
		return fmt.Errorf("connVersionExchange: could not prepare local version: %q", err)
	}

	// buffered channel to send our version just once
	outBuf := make(chan []byte, 1)
	outBuf <- myVersionMsg.Data()

	var gotTheirs, sendMine bool
	for {
		if gotTheirs && sendMine {
			break
		}

		select {
		case <-s.ctx.Done():
			// close Conn.
			remote.Close()
			return nil // BUG(cryptix): should this be an error?

		case <-remote.Closed:
			return errors.New("remote closed connection during version exchange")

		case our, ok := <-outBuf:
			if ok {
				remote.Secure.Out <- our
				sendMine = true
				close(outBuf) // only send local version once
				log.Debug("Send my version(%s) [to = %s]", myVersion, remote.Peer)
			}

		case data, ok := <-remote.Secure.In:
			if !ok {
				return fmt.Errorf("Error retrieving from conn: %v", remote.Peer)
			}

			remoteVersion = new(handshake.SemVer)
			err = proto.Unmarshal(data, remoteVersion)
			if err != nil {
				s.Close()
				return fmt.Errorf("connSetup: could not decode remote version: %q", err)
			}
			gotTheirs = true
			log.Debug("Received remote version(%s) [from = %s]", remoteVersion, remote.Peer)

			// BUG(cryptix): could add another case here to trigger resending our version
		}
	}

	if !handshake.Compatible(myVersion, remoteVersion) {
		remote.Close()
		return errors.New("protocol missmatch")
	}

	log.Debug("[peer: %s] Version compatible", remote.Peer)
	return nil
}

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	for {
		select {
		case <-s.ctx.Done():
			return // told to close.

		case msg, ok := <-s.Outgoing:
			if !ok {
				return
			}

			s.connsLock.RLock()
			conn, found := s.conns[msg.Peer().Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.errChan <- e
				continue
			}

			// log.Debug("[peer: %s] Sent message [to = %s]", s.local, msg.Peer())

			// queue it in the connection's buffer
			conn.Secure.Out <- msg.Data()
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanIn(c *conn.Conn) {
	for {
		select {
		case <-s.ctx.Done():
			// close Conn.
			c.Close()
			goto out

		case <-c.Closed:
			goto out

		case data, ok := <-c.Secure.In:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.Peer)
				s.errChan <- e
				goto out
			}

			// log.Debug("[peer: %s] Received message [from = %s]", s.local, c.Peer)

			msg := msg.New(c.Peer, data)
			s.Incoming <- msg
		}
	}

out:
	s.connsLock.Lock()
	delete(s.conns, c.Peer.Key())
	s.connsLock.Unlock()
}

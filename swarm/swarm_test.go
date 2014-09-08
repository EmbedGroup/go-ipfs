package swarm

import (
	"fmt"
	"net"
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	msgio "github.com/jbenet/go-msgio"
)

func pingListen(listener *net.TCPListener, peer *peer.Peer) {
	for {
		c, err := listener.Accept()
		if err == nil {
			fmt.Println("accepted")
			go pong(c, peer)
		}
	}
}

func pong(c net.Conn, peer *peer.Peer) {
	mrw := msgio.NewReadWriter(c)
	for {
		data := make([]byte, 1024)
		n, err := mrw.ReadMsg(data)
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		b, err := Unwrap(data[:n])
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		if string(b.GetMessage()) != "ping" {
			fmt.Printf("error: didn't receive ping: '%v'\n", b.GetMessage())
			return
		}

		data, err = Wrap([]byte("pong"), PBWrapper_TEST)
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		err = mrw.WriteMsg(data)
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
	}
}

func TestSwarm(t *testing.T) {

	swarm := NewSwarm(nil)
	var peers []*peer.Peer
	var listeners []net.Listener
	peerNames := map[string]string{
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a30": "/ip4/127.0.0.1/tcp/1234",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31": "/ip4/127.0.0.1/tcp/2345",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32": "/ip4/127.0.0.1/tcp/3456",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "/ip4/127.0.0.1/tcp/4567",
	}

	recv := swarm.GetChannel(PBWrapper_TEST)
	for k, n := range peerNames {
		peer, err := setupPeer(k, n)
		if err != nil {
			t.Fatal("error setting up peer", err)
		}
		a := peer.NetAddress("tcp")
		if a == nil {
			t.Fatal("error setting up peer (addr is nil)", peer)
		}
		n, h, err := a.DialArgs()
		if err != nil {
			t.Fatal("error getting dial args from addr")
		}
		listener, err := net.Listen(n, h)
		if err != nil {
			t.Fatal("error setting up listener", err)
		}
		go pingListen(listener.(*net.TCPListener), peer)

		conn, err, _ := swarm.Dial(peer)
		if err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}

		//Since we arent doing a handshake, set up 'secure' channels
		conn.secIn = conn.Incoming.MsgChan
		conn.secOut = conn.Outgoing.MsgChan

		swarm.StartConn(conn)
		// ok done, add it.
		peers = append(peers, peer)
		listeners = append(listeners, listener)
	}

	MsgNum := 1000
	for k := 0; k < MsgNum; k++ {
		for _, p := range peers {
			recv.Outgoing <- &Message{Peer: p, Data: []byte("ping")}
		}
	}

	got := map[u.Key]int{}

	for k := 0; k < (MsgNum * len(peers)); k++ {
		msg := <-recv.Incoming
		if string(msg.Data) != "pong" {
			t.Error("unexpected conn output", msg.Data)
		}

		n, _ := got[msg.Peer.Key()]
		got[msg.Peer.Key()] = n + 1
	}

	if len(peers) != len(got) {
		t.Error("got less messages than sent")
	}

	for p, n := range got {
		if n != MsgNum {
			t.Error("peer did not get all msgs", p, n, "/", MsgNum)
		}
	}

	fmt.Println("closing")
	swarm.Close()
	for _, listener := range listeners {
		listener.(*net.TCPListener).Close()
	}
}

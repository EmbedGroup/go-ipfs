package testutil

import (
	"bytes"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func RandKeyPair(bits int) (ci.PrivKey, ci.PubKey, error) {
	return ci.GenerateKeyPair(ci.RSA, bits, crand.Reader)
}

func SeededKeyPair(bits int, seed int64) (ci.PrivKey, ci.PubKey, error) {
	return ci.GenerateKeyPair(ci.RSA, bits, u.NewSeededRand(seed))
}

// RandPeerID generates random "valid" peer IDs. it does not NEED to generate
// keys because it is as if we lost the key right away. fine to read randomness
// and hash it. to generate proper keys and corresponding PeerID, use:
//  sk, pk, _ := testutil.RandKeyPair()
//  id, _ := peer.IDFromPublicKey(pk)
func RandPeerID() (peer.ID, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(crand.Reader, buf); err != nil {
		return "", err
	}
	h := u.Hash(buf)
	return peer.ID(h), nil
}

func RandPeerIDFatal(t testing.TB) peer.ID {
	p, err := RandPeerID()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// RandLocalTCPAddress returns a random multiaddr. it suppresses errors
// for nice composability-- do check the address isn't nil.
func RandLocalTCPAddress() ma.Multiaddr {

	// chances are it will work out, but it **might** fail if the port is in use
	// most ports above 10000 aren't in use by long running processes, so yay.
	// (maybe there should be a range of "loopback" ports that are guaranteed
	// to be open for the process, but naturally can only talk to self.)

	lastPort.Lock()
	if lastPort.port == 0 {
		lastPort.port = 10000 + SeededRand.Intn(50000)
	}
	port := lastPort.port
	lastPort.port++
	lastPort.Unlock()

	addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	maddr, _ := ma.NewMultiaddr(addr)
	return maddr
}

var lastPort = struct {
	port int
	sync.Mutex
}{}

// PeerNetParams is a struct to bundle together the four things
// you need to run a connection with a peer: id, 2keys, and addr.
type PeerNetParams struct {
	ID      peer.ID
	PrivKey ci.PrivKey
	PubKey  ci.PubKey
	Addr    ma.Multiaddr
}

func (p *PeerNetParams) checkKeys() error {
	if !p.ID.MatchesPrivateKey(p.PrivKey) {
		return errors.New("p.ID does not match p.PrivKey")
	}

	if !p.ID.MatchesPublicKey(p.PubKey) {
		return errors.New("p.ID does not match p.PubKey")
	}

	var buf bytes.Buffer
	buf.Write([]byte("hello world. this is me, I swear."))
	b := buf.Bytes()

	sig, err := p.PrivKey.Sign(b)
	if err != nil {
		return fmt.Errorf("sig signing failed: %s", err)
	}

	sigok, err := p.PubKey.Verify(b, sig)
	if err != nil {
		return fmt.Errorf("sig verify failed: %s", err)
	}
	if !sigok {
		return fmt.Errorf("sig verify failed: sig invalid!")
	}

	return nil // ok. move along.
}

func RandPeerNetParamsOrFatal(t *testing.T) PeerNetParams {
	p, err := RandPeerNetParams()
	if err != nil {
		t.Fatal(err)
		return PeerNetParams{} // TODO return nil
	}
	return *p
}

func RandPeerNetParams() (*PeerNetParams, error) {
	var p PeerNetParams
	var err error
	p.Addr = RandLocalTCPAddress()
	p.PrivKey, p.PubKey, err = ci.GenerateKeyPair(ci.RSA, 512, u.NewTimeSeededRand())
	if err != nil {
		return nil, err
	}
	p.ID, err = peer.IDFromPublicKey(p.PubKey)
	if err != nil {
		return nil, err
	}
	if err := p.checkKeys(); err != nil {
		return nil, err
	}
	return &p, nil
}

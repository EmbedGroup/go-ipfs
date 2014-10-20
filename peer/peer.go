package peer

import (
	"fmt"
	"sync"
	"time"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	ic "github.com/jbenet/go-ipfs/crypto"
	u "github.com/jbenet/go-ipfs/util"

	"bytes"
)

var log = u.Logger("peer")

// ID is a byte slice representing the identity of a peer.
type ID mh.Multihash

// String is utililty function for printing out peer ID strings.
func (id ID) String() string {
	return id.Pretty()
}

// Equal is utililty function for comparing two peer ID's
func (id ID) Equal(other ID) bool {
	return bytes.Equal(id, other)
}

// Pretty returns a b58-encoded string of the ID
func (id ID) Pretty() string {
	return b58.Encode(id)
}

// DecodePrettyID returns a b58-encoded string of the ID
func DecodePrettyID(s string) ID {
	return b58.Decode(s)
}

// IDFromPubKey retrieves a Public Key from the peer given by pk
func IDFromPubKey(pk ic.PubKey) (ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return nil, err
	}
	hash := u.Hash(b)
	return ID(hash), nil
}

// Map maps Key (string) : *Peer (slices are not comparable).
type Map map[u.Key]*Peer

// Peer represents the identity information of an IPFS Node, including
// ID, and relevant Addresses.
type Peer struct {
	ID        ID
	Addresses []ma.Multiaddr

	PrivKey ic.PrivKey
	PubKey  ic.PubKey

	latency time.Duration

	sync.RWMutex
}

// String prints out the peer.
func (p *Peer) String() string {
	return "[Peer " + p.ID.String()[:12] + "]"
}

// Key returns the ID as a Key (string) for maps.
func (p *Peer) Key() u.Key {
	return u.Key(p.ID)
}

// AddAddress adds the given Multiaddr address to Peer's addresses.
func (p *Peer) AddAddress(a ma.Multiaddr) {
	p.Lock()
	defer p.Unlock()

	for _, addr := range p.Addresses {
		if addr.Equal(a) {
			return
		}
	}
	p.Addresses = append(p.Addresses, a)
}

// NetAddress returns the first Multiaddr found for a given network.
func (p *Peer) NetAddress(n string) ma.Multiaddr {
	p.RLock()
	defer p.RUnlock()

	for _, a := range p.Addresses {
		for _, p := range a.Protocols() {
			if p.Name == n {
				return a
			}
		}
	}
	return nil
}

// GetLatency retrieves the current latency measurement.
func (p *Peer) GetLatency() (out time.Duration) {
	p.RLock()
	out = p.latency
	p.RUnlock()
	return
}

// SetLatency sets the latency measurement.
// TODO: Instead of just keeping a single number,
//		 keep a running average over the last hour or so
// Yep, should be EWMA or something. (-jbenet)
func (p *Peer) SetLatency(laten time.Duration) {
	p.Lock()
	if p.latency == 0 {
		p.latency = laten
	} else {
		p.latency = ((p.latency * 9) + laten) / 10
	}
	p.Unlock()
}

// LoadAndVerifyKeyPair unmarshalls, loads a private/public key pair.
// Error if (a) unmarshalling fails, or (b) pubkey does not match id.
func (p *Peer) LoadAndVerifyKeyPair(marshalled []byte) error {

	sk, err := ic.UnmarshalPrivateKey(marshalled)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal private key: %v", err)
	}

	// construct and assign pubkey. ensure it matches this peer
	if err := p.VerifyAndSetPubKey(sk.GetPublic()); err != nil {
		return err
	}

	// if we didn't have the priavte key, assign it
	if p.PrivKey == nil {
		p.PrivKey = sk
		return nil
	}

	// if we already had the keys, check they're equal.
	if p.PrivKey.Equals(sk) {
		return nil // as expected. keep the old objects.
	}

	// keys not equal. invariant violated. this warrants a panic.
	// these keys should be _the same_ because peer.ID = H(pk)
	// this mismatch should never happen.
	log.Error("%s had PrivKey: %v -- got %v", p, p.PrivKey, sk)
	panic("invariant violated: unexpected key mismatch")
}

// VerifyAndSetPubKey sets public key, given it matches the peer.ID
func (p *Peer) VerifyAndSetPubKey(pk ic.PubKey) error {
	pkid, err := IDFromPubKey(pk)
	if err != nil {
		return fmt.Errorf("Failed to hash public key: %v", err)
	}

	if !p.ID.Equal(pkid) {
		return fmt.Errorf("Public key does not match peer.ID.")
	}

	// if we didn't have the keys, assign them.
	if p.PubKey == nil {
		p.PubKey = pk
		return nil
	}

	// if we already had the pubkey, check they're equal.
	if p.PubKey.Equals(pk) {
		return nil // as expected. keep the old objects.
	}

	// keys not equal. invariant violated. this warrants a panic.
	// these keys should be _the same_ because peer.ID = H(pk)
	// this mismatch should never happen.
	log.Error("%s had PubKey: %v -- got %v", p, p.PubKey, pk)
	panic("invariant violated: unexpected key mismatch")
}

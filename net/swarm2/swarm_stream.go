package swarm

import (
	ps "github.com/jbenet/go-peerstream"
)

// a Stream is a wrapper around a ps.Stream that exposes a way to get
// our SwarmConn and Swarm (instead of just the ps.Conn and ps.Swarm)
type Stream ps.Stream

// StreamHandler is called when new streams are opened from remote peers.
// See peerstream.StreamHandler
type StreamHandler func(*Stream)

// Stream returns the underlying peerstream.Stream
func (s *Stream) Stream() *ps.Stream {
	return (*ps.Stream)(s)
}

// Conn returns the Conn associated with this Stream
func (s *Stream) Conn() *SwarmConn {
	return (*SwarmConn)(s.Stream().Conn())
}

// Write writes bytes to a stream, calling write data for each call.
func (s *Stream) Wait() error {
	return s.Stream().Wait()
}

func (s *Stream) Read(p []byte) (n int, err error) {
	return s.Stream().Read(p)
}

func (s *Stream) Write(p []byte) (n int, err error) {
	return s.Stream().Write(p)
}

func (s *Stream) Close() error {
	return s.Stream().Close()
}

func wrapStream(pss *ps.Stream) *Stream {
	return (*Stream)(pss)
}

func wrapStreams(st []*ps.Stream) []*Stream {
	out := make([]*Stream, len(st))
	for i, s := range st {
		out[i] = wrapStream(s)
	}
	return out
}

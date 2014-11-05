package util

import "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

func Do(ctx context.Context, f func() error) error {

	ch := make(chan error)

	go func() {
		select {
		case <-ctx.Done():
		case ch <- f():
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case val := <-ch:
		return val
	}
	return nil
}

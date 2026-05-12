package adapters

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"
)

// ErrDrainTimedOut is returned by Draining when work did not exit
// within the drain timeout and was force-cancelled.
var ErrDrainTimedOut = errors.New("adapters: drain timed out")

type stoppingKey struct{}

// Stopping returns a channel that closes when Draining begins shutdown,
// or nil when ctx is not inside a Draining adapter. Always also select
// on ctx.Done() — Stopping signals drain start; ctx.Done() fires only
// when the drain timer expires.
func Stopping(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(stoppingKey{}).(<-chan struct{})
	return ch
}

// Draining wraps work with graceful-shutdown semantics: when outerCtx
// is cancelled, work has up to timeout to return via Stopping(workCtx)
// before workCtx is force-cancelled and ErrDrainTimedOut is returned.
//
// Panics in work are recovered inside Draining's goroutine and returned
// as an error containing the panic value and stack trace. They do NOT
// propagate to runnable.WithRecoverer — recover only fires on the
// goroutine where the deferred call lives, and work runs on its own.
func Draining(timeout time.Duration, work func(context.Context) error) func(context.Context) error {
	return func(outerCtx context.Context) error {
		// Decoupled from outerCtx so outer cancellation triggers drain
		// rather than aborting work directly.
		workCtx, cancelWork := context.WithCancel(context.WithoutCancel(outerCtx))
		defer cancelWork()

		stopping := make(chan struct{})
		workCtx = context.WithValue(workCtx, stoppingKey{}, (<-chan struct{})(stopping))

		done := make(chan error, 1)
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					done <- fmt.Errorf("adapters: panic in draining work: %v\n%s", rec, debug.Stack())
				}
			}()
			done <- work(workCtx)
		}()

		select {
		case err := <-done:
			return err
		case <-outerCtx.Done():
			close(stopping)
		}

		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-done:
			return err
		case <-timer.C:
			cancelWork()
			<-done
			return ErrDrainTimedOut
		}
	}
}

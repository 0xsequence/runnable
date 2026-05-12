package adapters

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/0xsequence/runnable"
)

// ErrDrainTimedOut is returned by Draining when work did not exit
// within the drain timeout and was force-cancelled.
var ErrDrainTimedOut = errors.New("adapters: drain timed out")

type stoppingKey struct{}

// Stopping returns a channel that closes when Draining begins shutdown,
// or nil outside Draining. Select on this alongside ctx.Done() — ctx is
// force-cancelled only after the drain timer expires.
func Stopping(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(stoppingKey{}).(<-chan struct{})
	return ch
}

// Draining returns an Adapter that delays cancellation: when outerCtx
// is cancelled, next has up to timeout to return via Stopping(workCtx)
// before workCtx is force-cancelled and ErrDrainTimedOut is returned.
// Panics in next are recovered into an error (they would otherwise
// crash the process, since next runs on its own goroutine).
func Draining(timeout time.Duration) runnable.Adapter {
	return func(next runnable.RunFunc) runnable.RunFunc {
		return func(outerCtx context.Context) error {
			// Decoupled from outerCtx so outer cancellation triggers drain
			// rather than aborting next directly.
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
				done <- next(workCtx)
			}()

			select {
			case err := <-done:
				return err
			case <-outerCtx.Done():
				runnable.Publish(outerCtx, runnable.DrainStartedEvent{Timeout: timeout})
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
				runnable.Publish(outerCtx, runnable.DrainTimedOutEvent{})
				return ErrDrainTimedOut
			}
		}
	}
}

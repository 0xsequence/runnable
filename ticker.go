package runnable

import (
	"context"
	"time"
)

// NewTicker returns a Runnable that calls tick once per interval until
// ctx is cancelled, Stop is called, or tick returns a non-nil error.
//
// When the Runnable is configured WithDrain, an in-flight tick is
// allowed to finish before Run returns; the loop exits without firing
// a new tick. Without WithDrain, Stop cancels ctx and any in-flight
// tick observes the cancellation through ctx.Done().
//
// tick should respect ctx.Done() for cancellation. To make in-flight
// external calls survive shutdown under WithDrain, tick should derive
// per-call timeouts via context.WithoutCancel(ctx) so its work is not
// affected by either Stop's drain signal or the Runnable's ctx cancel.
//
// Composing with WithRetry resets the ticker cadence on every retry:
// a tick error bails the loop, WithRetry re-enters runFunc, and the
// next tick fires `interval` after the retry — not at the original
// cadence. If you need stable cadence with transient-error tolerance,
// handle retries inside `tick` instead.
func NewTicker(interval time.Duration, tick func(ctx context.Context) error, opts ...Option) Runnable {
	return New(func(ctx context.Context) error {
		t := time.NewTicker(interval)
		defer t.Stop()

		stopping := Stopping(ctx) // nil when WithDrain not used

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-stopping:
				return nil
			case <-t.C:
				// Re-check shutdown signals before firing a new tick:
				// when a tick takes longer than the interval, t.C may
				// have ready ticks queued from before Stop was called.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-stopping:
					return nil
				default:
				}
				if err := tick(ctx); err != nil {
					return err
				}
			}
		}
	}, opts...)
}

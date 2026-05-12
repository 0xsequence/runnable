package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/0xsequence/runnable"
)

// ResetNever (as resetAfter) disables retry-budget reset.
const ResetNever time.Duration = 0

// Retry returns an Adapter that re-invokes next up to maxRetries times
// on non-context errors. If resetAfter > 0 and at least that long has
// passed since the previous attempt, the budget resets. Each failed
// attempt publishes a runnable.RetryEvent (Attempt is 1-indexed). Retry
// does not observe Stopping — wrap it inside Draining if you need both.
func Retry(maxRetries int, resetAfter time.Duration) runnable.Adapter {
	return func(next runnable.RunFunc) runnable.RunFunc {
		return func(ctx context.Context) error {
			// lastTime is per-call: the timer for reset budgets is local
			// to this invocation, not shared across runnable cycles.
			var lastTime time.Time
			var err error
			for i := 0; i < maxRetries; i++ {
				if resetAfter != ResetNever && time.Since(lastTime) > resetAfter {
					i = 0
				}
				lastTime = time.Now()

				err = next(ctx)
				if err == nil {
					return nil
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				runnable.Publish(ctx, runnable.RetryEvent{Attempt: i + 1, Err: err})
			}
			return err
		}
	}
}

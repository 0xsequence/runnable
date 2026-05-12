package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/0xsequence/runnable"
)

// ResetNever passed as resetAfter disables retry-budget reset based on
// elapsed time between attempts.
const ResetNever time.Duration = 0

// Retry returns an Adapter that re-invokes next up to maxRetries times
// when it returns a non-nil, non-context error. If resetAfter is
// non-zero and at least that long has passed since the previous
// attempt, the retry budget resets.
//
// Retry never observes Draining's Stopping signal — drain semantics
// belong to Draining alone. Compose with Draining outside Retry if you
// need both.
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
			}
			return err
		}
	}
}

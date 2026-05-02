package runnable

import (
	"context"
	"errors"
	"time"
)

const ResetNever time.Duration = 0

type withRetry struct {
	maxRetries int
	resetAfter time.Duration
}

func WithRetry(maxRetries int, resetAfter time.Duration) Option {
	return &withRetry{
		maxRetries: maxRetries,
		resetAfter: resetAfter,
	}
}

func (w *withRetry) apply(r *runnable) {
	runFunc := r.runFunc
	r.runFunc = func(ctx context.Context) error {
		// lastTime is per-Run-cycle: a fresh Run after Stop should not
		// inherit stale timing state from the prior cycle.
		var lastTime time.Time
		var err error
		for i := 0; i < w.maxRetries; i++ {
			if w.resetAfter != ResetNever && time.Since(lastTime) > w.resetAfter {
				i = 0
			}
			lastTime = time.Now()

			if i > 0 {
				if r.onStart != nil {
					r.onStart()
				}
			}

			err = runFunc(ctx)
			if err == nil {
				return nil
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			// Don't retry once Stop has been called via WithDrain —
			// the retry wrapper would otherwise re-enter runFunc and
			// start fresh work mid-shutdown, defeating drain semantics.
			// When WithDrain is not used, Stopping(ctx) is nil and the
			// default branch runs (no behavior change).
			select {
			case <-Stopping(ctx):
				return err
			default:
			}

			if i > 0 {
				if r.onStop != nil {
					r.onStop()
				}
			}
		}
		return err
	}
}

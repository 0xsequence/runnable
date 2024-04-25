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

	lastTime time.Time
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
		var err error
		for i := 0; i < w.maxRetries; i++ {
			if w.resetAfter != ResetNever && time.Since(w.lastTime) > w.resetAfter {
				i = 0
			}
			w.lastTime = time.Now()

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

			if i > 0 {
				if r.onStop != nil {
					r.onStop()
				}
			}
		}
		return err
	}
}

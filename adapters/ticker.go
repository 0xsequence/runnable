package adapters

import (
	"context"
	"time"
)

// Ticker calls tick once per interval until ctx is cancelled or tick
// returns an error. Composes with Draining: an in-flight tick is
// allowed to finish before the loop exits.
func Ticker(interval time.Duration, tick func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		t := time.NewTicker(interval)
		defer t.Stop()
		stopping := Stopping(ctx)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-stopping:
				return nil
			case <-t.C:
				// Re-check shutdown signals: queued ticks during a slow
				// tick can race against stopping in select's random pick.
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
	}
}

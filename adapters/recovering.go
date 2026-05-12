package adapters

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/0xsequence/runnable"
)

// Recovering returns an Adapter that converts panics from next into
// errors and publishes a runnable.PanicRecoveredEvent to the Publisher
// on ctx, if any. Place inside Draining when both are used, so the
// Publisher sees the panic before Draining's safety-net recovery
// formats it.
func Recovering() runnable.Adapter {
	return func(next runnable.RunFunc) runnable.RunFunc {
		return func(ctx context.Context) (err error) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				stack := debug.Stack()
				runnable.Publish(ctx, runnable.PanicRecoveredEvent{Recovered: rec, Stack: stack})
				err = fmt.Errorf("adapters: panic: %v", rec)
			}()
			return next(ctx)
		}
	}
}

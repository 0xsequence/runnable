package adapters

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/0xsequence/runnable"
)

// PanicHandler observes a panic caught by Recovering. Runs on next's
// goroutine, so must not block.
type PanicHandler func(ctx context.Context, rec any, stack []byte)

// Recovering returns an Adapter that converts panics from next into
// errors and invokes handler (if non-nil). Place inside Draining when
// both are used, so handler sees the panic before Draining's safety-net
// recovery formats it.
func Recovering(handler PanicHandler) runnable.Adapter {
	return func(next runnable.RunFunc) runnable.RunFunc {
		return func(ctx context.Context) (err error) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				stack := debug.Stack()
				if handler != nil {
					handler(ctx, rec, stack)
				}
				err = fmt.Errorf("adapters: panic: %v", rec)
			}()
			return next(ctx)
		}
	}
}

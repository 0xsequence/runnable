package adapters

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/0xsequence/runnable"
)

// PanicHandler is invoked when Recovering catches a panic, before the
// adapter converts it into an error. It runs on the same goroutine as
// next, so handlers must not block.
type PanicHandler func(ctx context.Context, rec any, stack []byte)

// Recovering returns an Adapter that catches panics from next and
// converts them into errors. handler is optional; pass nil to skip
// reporting.
//
// Place Recovering inside Draining when both are in use: Draining
// already recovers panics in its own goroutine as a safety net, but
// Recovering inside lets the handler observe the panic before
// Draining's generic recovery formats it.
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

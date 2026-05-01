package runnable

import (
	"context"
	"time"
)

type stoppingKey struct{}

// Stopping returns a channel that closes when Stop has been called on
// the Runnable that owns ctx. runFunc implementations under WithDrain
// should select on it and return cleanly without cancelling in-flight
// work.
//
// Always also select on ctx.Done() — Stopping signals only Stop;
// outer-context cancellation (e.g. the ctx passed to Run was cancelled
// directly) still arrives via ctx.Done(). A loop that selects only on
// Stopping(ctx) will hang on outer-ctx cancel.
//
// Returns a nil channel when ctx is not associated with a drain-enabled
// Runnable — receiving from a nil channel blocks forever, which is the
// correct no-op for callers that opt into drain semantics only when
// configured.
func Stopping(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(stoppingKey{}).(<-chan struct{})
	return ch
}

type withDrain struct {
	timeout time.Duration
}

// WithDrain switches Stop's behavior from "cancel runFunc's ctx" to
// "close Stopping(ctx) and wait up to timeout for runFunc to return on
// its own." After the timeout elapses, Stop falls back to cancelling
// the ctx as before (preserving the existing escape hatch for stuck
// runFuncs). Use this when runFunc owns in-flight external calls that
// must drain rather than abort on shutdown.
func WithDrain(timeout time.Duration) Option {
	return &withDrain{timeout: timeout}
}

func (w *withDrain) apply(r *runnable) {
	r.drainEnabled = true
	r.drainTimeout = w.timeout
}

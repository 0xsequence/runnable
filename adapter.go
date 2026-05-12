package runnable

import "context"

// RunFunc is the signature wrapped by runnable.New and by Adapters.
type RunFunc func(ctx context.Context) error

// Adapter wraps a RunFunc with cross-cutting behavior, mirroring the
// chi middleware shape. Adapters live in the runnable/adapters
// subpackage and are applied via WithAdapters.
type Adapter func(next RunFunc) RunFunc

type withAdapters struct {
	adapters []Adapter
}

// WithAdapters wraps the runnable's runFunc with the given adapters.
// Adapters are applied left-to-right as outermost-to-innermost:
// WithAdapters(A, B, C) yields A(B(C(runFunc))).
//
// Apply order across Options matters: WithAdapters sees whatever
// runFunc previous Options installed, and subsequent Options see the
// adapter-wrapped runFunc.
func WithAdapters(adapters ...Adapter) Option {
	return &withAdapters{adapters: adapters}
}

func (w *withAdapters) apply(r *runnable) {
	next := RunFunc(r.runFunc)
	for i := len(w.adapters) - 1; i >= 0; i-- {
		next = w.adapters[i](next)
	}
	r.runFunc = next
}

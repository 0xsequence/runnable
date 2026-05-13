package runnable

import "context"

// RunFunc is the lifecycle function wrapped by runnable.New.
type RunFunc func(ctx context.Context) error

// Adapter wraps a RunFunc with cross-cutting behavior, mirroring the
// chi middleware shape. Concrete adapters live in runnable/adapters.
type Adapter func(next RunFunc) RunFunc

type withAdapters struct {
	adapters []Adapter
}

// WithAdapters wraps the runnable's runFunc left-to-right (first listed
// = outermost). Apply order across Options matters.
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

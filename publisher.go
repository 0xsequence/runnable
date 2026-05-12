package runnable

import "context"

// Publisher receives events from adapters that opt in to publishing.
// Implementations must not block; if a subscriber needs async dispatch,
// it should buffer internally.
type Publisher interface {
	Publish(event any)
}

// Publishers is a fanout Publisher that forwards each event to every
// member in order. Nil entries are skipped.
type Publishers []Publisher

func (ps Publishers) Publish(event any) {
	for _, p := range ps {
		if p != nil {
			p.Publish(event)
		}
	}
}

type publisherKey struct{}

// PublisherFrom returns the Publisher installed in ctx by runnable.Run,
// or nil if none. Adapters should prefer Publish, which no-ops when no
// Publisher is set.
func PublisherFrom(ctx context.Context) Publisher {
	p, _ := ctx.Value(publisherKey{}).(Publisher)
	return p
}

// Publish forwards event to the Publisher in ctx, or no-ops if none.
// Use this from adapters that emit observability events.
func Publish(ctx context.Context, event any) {
	if p := PublisherFrom(ctx); p != nil {
		p.Publish(event)
	}
}

type withPublisher struct {
	p Publisher
}

// WithPublisher installs p as the runnable's event Publisher. Stacks
// additively across multiple WithPublisher (and WithStatus) Options:
// subsequent installs fan out to all prior subscribers.
func WithPublisher(p Publisher) Option {
	return &withPublisher{p: p}
}

func (w *withPublisher) apply(r *runnable) {
	r.publisher = appendPublisher(r.publisher, w.p)
}

func appendPublisher(existing, next Publisher) Publisher {
	if next == nil {
		return existing
	}
	if existing == nil {
		return next
	}
	if ps, ok := existing.(Publishers); ok {
		return append(ps, next)
	}
	return Publishers{existing, next}
}

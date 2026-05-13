package runnable

import "context"

// Publisher receives events from adapters. Implementations must not
// block — buffer internally if async dispatch is needed.
type Publisher interface {
	Publish(event any)
}

// Publishers fans out each event to every member in order; nil members
// are skipped.
type Publishers []Publisher

func (ps Publishers) Publish(event any) {
	for _, p := range ps {
		if p != nil {
			p.Publish(event)
		}
	}
}

type publisherKey struct{}

// PublisherFrom returns the Publisher installed in ctx, or nil. Adapters
// should prefer Publish, which no-ops when none is set.
func PublisherFrom(ctx context.Context) Publisher {
	p, _ := ctx.Value(publisherKey{}).(Publisher)
	return p
}

// Publish forwards event to the Publisher in ctx, or no-ops if none.
func Publish(ctx context.Context, event any) {
	if p := PublisherFrom(ctx); p != nil {
		p.Publish(event)
	}
}

type withPublisher struct {
	p Publisher
}

// WithPublisher installs p as the runnable's event Publisher. Stacks
// additively across calls (and with WithStatus).
func WithPublisher(p Publisher) Option {
	return &withPublisher{p: p}
}

func (w *withPublisher) apply(r *runnable) {
	r.publisher = mergePublisher(r.publisher, w.p)
}

func mergePublisher(existing, next Publisher) Publisher {
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

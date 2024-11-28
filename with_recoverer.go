package runnable

import (
	"context"
	"fmt"
)

type RecoveryReporter interface {
	Report(ctx context.Context, rec interface{})
}

// NoopReporter
// Used to continue running go routine  and do nothing
type NoopReporter struct{}

func (*NoopReporter) Report(ctx context.Context, rec interface{}) {}

type recoverer struct {
	reporter RecoveryReporter
}

func WithRecoverer(reporter RecoveryReporter) Option {
	return &recoverer{
		reporter: reporter,
	}
}

func (rec *recoverer) apply(r *runnable) {
	originalRunFunc := r.runFunc
	r.runFunc = func(ctx context.Context) error {
		var err error
		innerRun := func(ctx context.Context) error {
			defer func() {
				if recovery := recover(); recovery != nil {
					err = fmt.Errorf("panic: %v", recovery)

					if rec.reporter != nil {
						rec.reporter.Report(ctx, recovery)
					}
				}
			}()

			return originalRunFunc(ctx)
		}

		if errInner := innerRun(ctx); errInner != nil {
			return errInner
		}

		return err
	}
}

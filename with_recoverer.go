package runnable

import (
	"context"
)

type RecoveryReporter interface {
	Report(ctx context.Context, runnableId string, rec interface{})
}

// NoopReporter
// Used to continue running go routine  and do nothing
type NoopReporter struct{}

func (*NoopReporter) Report(ctx context.Context, runnableId string, rec interface{}) {}

type recoverer struct {
	runnableId string
	reporter   RecoveryReporter
}

func WithRecoverer(runnableId string, reporter RecoveryReporter) Option {
	return &recoverer{
		runnableId: runnableId,
		reporter:   reporter,
	}
}

func (rec *recoverer) apply(r *runnable) {
	runFunc := r.runFunc
	r.runFunc = func(ctx context.Context) error {
		var err error
		err = func() error {
			defer func() {
				if err := recover(); err != nil {
					rec.reporter.Report(ctx, rec.runnableId, err)
				}
			}()

			return runFunc(ctx)
		}()

		return err
	}
}

package runnable

import (
	"context"
	"fmt"
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
		innerRun := func(ctx context.Context) error {
			defer func() {
				if recovery := recover(); recovery != nil {
					err = fmt.Errorf("panic: %v", recovery)
					rec.reporter.Report(ctx, rec.runnableId, recovery)
				}
			}()

			return runFunc(ctx)
		}

		errDirect := innerRun(ctx)
		if errDirect != nil {
			return errDirect
		}

		return err
	}
}

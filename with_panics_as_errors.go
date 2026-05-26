package runnable

import (
	"context"
	"fmt"
)

type withPanicsAsErrors struct {
}

func WithPanicsAsErrors() Option {
	return &withPanicsAsErrors{}
}

func (w *withPanicsAsErrors) apply(r *runnable) {
	runFunc := r.runFunc
	r.runFunc = func(ctx context.Context) error {
		var err error
		innerRun := func(ctx context.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
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

package runnable

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// NewGroup creates a new Runnable that runs multiple runnables concurrently.
//
// Example:
//
//	group := NewGroup(
//		New(func(ctx context.Context) error {
//			// do something
//			return nil
//		}),
//		New(func(ctx context.Context) error {
//			// do something
//			return nil
//		}),
//	)
//
//	err := group.Run(context.Background())
//	if err != nil {
//		// handle error
//	}
func NewGroup(runners ...Runnable) Runnable {
	return New(func(ctx context.Context) error {
		grp, groupCtx := errgroup.WithContext(ctx)
		for _, r := range runners {
			r := r
			grp.Go(func() error {
				return r.Run(groupCtx)
			})
		}
		return grp.Wait()
	})
}

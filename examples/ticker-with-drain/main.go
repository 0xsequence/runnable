// Example: a periodic reconciler that drains gracefully on SIGTERM.
//
// Shape: adapters.Draining + adapters.Ticker + signal.NotifyContext.
// Copy-paste into a service's cmd/.../main.go and replace the reconcile
// body with your work.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

func reconcile(ctx context.Context) error {
	// Pretend this is an HTTP call to an external system that must not
	// be aborted mid-request when SIGTERM fires. Under Draining, this
	// tick is allowed to finish before the runnable tears down.
	fmt.Println("tick: reconciling...")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("tick: done")
	return nil
}

func main() {
	sigCtx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSig()

	// Unlike v0.1, passing sigCtx directly to Run is safe: the
	// Draining adapter intercepts cancellation and triggers drain
	// rather than aborting work.
	//
	// Note on panics: a panic inside reconcile is recovered by
	// Draining and surfaced as an error from rc.Run. runnable.WithRecoverer
	// does NOT catch tick panics in this composition (recover only fires
	// on the goroutine where the deferred call lives, and Draining runs
	// work on its own goroutine). Read panics off runErr below.
	rc := runnable.New(
		adapters.Draining(10*time.Second,
			adapters.Ticker(2*time.Second, reconcile),
		),
	)

	runErr := make(chan error, 1)
	go func() {
		runErr <- rc.Run(sigCtx)
	}()

	select {
	case <-sigCtx.Done():
		fmt.Println("shutdown: draining in-flight tick...")
		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := rc.Stop(stopCtx); err != nil {
			fmt.Fprintf(os.Stderr, "stop: %v\n", err)
		}
		if err := <-runErr; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, adapters.ErrDrainTimedOut) {
			fmt.Fprintf(os.Stderr, "reconciler stopped: %v\n", err)
			os.Exit(1)
		}
	case err := <-runErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "reconciler stopped: %v\n", err)
			os.Exit(1)
		}
	}
}

// Example: a periodic reconciler that drains gracefully on SIGTERM.
//
// Shape: runnable.WithAdapters composing Draining + Recovering + Ticker,
// driven by signal.NotifyContext. Copy-paste into a service's
// cmd/.../main.go and replace the reconcile body with your work.
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

func panicHandler(_ context.Context, rec any, stack []byte) {
	fmt.Fprintf(os.Stderr, "tick panic: %v\n%s", rec, stack)
}

func main() {
	sigCtx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSig()

	// Adapters compose left-to-right (first listed = outermost). Draining
	// catches outer ctx cancellation and turns it into drain rather than
	// abort. Recovering sits inside Draining so the handler observes
	// panics before Draining's safety-net recovery formats them.
	rc := runnable.New(reconcile, runnable.WithAdapters(
		adapters.Draining(10*time.Second),
		adapters.Recovering(panicHandler),
		adapters.Ticker(2*time.Second),
	))

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

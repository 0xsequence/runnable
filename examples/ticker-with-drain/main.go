// Example: a periodic reconciler that drains gracefully on SIGTERM.
//
// Shape: runnable.WithAdapters composing Draining + Recovering + Ticker,
// driven by signal.NotifyContext. A Publisher subscribes to adapter
// events. Copy-paste into a service's cmd/.../main.go and replace the
// reconcile body with your work.
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

// logPublisher prints each adapter event. In a real service this would
// push to a metrics or structured-logging sink.
type logPublisher struct{}

func (logPublisher) Publish(event any) {
	switch ev := event.(type) {
	case runnable.PanicRecoveredEvent:
		fmt.Fprintf(os.Stderr, "tick panic: %v\n%s", ev.Recovered, ev.Stack)
	case runnable.DrainStartedEvent:
		fmt.Printf("drain: started, %s grace window\n", ev.Timeout)
	case runnable.DrainTimedOutEvent:
		fmt.Println("drain: timed out, force-cancelled")
	case runnable.RetryEvent:
		fmt.Printf("retry: attempt %d failed: %v\n", ev.Attempt, ev.Err)
	}
}

func main() {
	sigCtx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSig()

	// Adapters compose left-to-right (first listed = outermost). Draining
	// catches outer ctx cancellation and turns it into drain rather than
	// abort. Recovering sits inside Draining so the Publisher observes
	// the panic before Draining's safety-net recovery formats it.
	rc := runnable.New(reconcile,
		runnable.WithPublisher(logPublisher{}),
		runnable.WithAdapters(
			adapters.Draining(10*time.Second),
			adapters.Recovering(),
			adapters.Ticker(2*time.Second),
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

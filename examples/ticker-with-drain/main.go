// Example: a periodic reconciler that drains gracefully on SIGTERM.
//
// Shape: NewTicker + WithDrain + WithRecoverer + signal.NotifyContext.
// Copy-paste this into a service's cmd/.../main.go and replace the
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
)

type stderrReporter struct{}

func (stderrReporter) Report(ctx context.Context, rec interface{}) {
	fmt.Fprintf(os.Stderr, "panic recovered: %v\n", rec)
}

type stderrPrinter struct{}

func (stderrPrinter) Print(ctx context.Context, callstack []byte) {
	_, _ = os.Stderr.Write(callstack)
}

func reconcile(ctx context.Context) error {
	// Pretend this is an HTTP call to an external system that must not
	// be aborted mid-request when SIGTERM fires. Under WithDrain, Stop
	// waits for this tick to finish before tearing down the Runnable.
	fmt.Println("tick: reconciling...")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("tick: done")
	return nil
}

func main() {
	sigCtx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSig()

	rc := runnable.NewTicker(
		2*time.Second,
		reconcile,
		runnable.WithDrain(10*time.Second),
		runnable.WithRecoverer(stderrReporter{}, stderrPrinter{}),
	)

	// Run with a pristine ctx — if Run received sigCtx, SIGTERM would
	// cancel runFunc's ctx directly and the ticker would exit before
	// Stop ever closed Stopping(ctx), defeating WithDrain. Stop is the
	// only thing that should drive shutdown of a drain-enabled worker.
	runErr := make(chan error, 1)
	go func() {
		runErr <- rc.Run(context.Background())
	}()

	<-sigCtx.Done()
	fmt.Println("shutdown: draining in-flight tick...")

	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := rc.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
	}

	if err := <-runErr; err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "reconciler stopped: %v\n", err)
		os.Exit(1)
	}
}

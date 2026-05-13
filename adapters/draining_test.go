package adapters_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

func TestStopping_NilOutsideDraining(t *testing.T) {
	ch := adapters.Stopping(context.Background())
	assert.Nil(t, ch, "Stopping(ctx) must be nil when ctx is not inside a Draining adapter")
}

func TestDraining_WorkReturnsNaturallyViaStopping(t *testing.T) {
	started := make(chan struct{})

	work := func(ctx context.Context) error {
		close(started)
		select {
		case <-adapters.Stopping(ctx):
			return nil
		case <-ctx.Done():
			return errors.New("ctx cancelled before Stopping")
		}
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(1*time.Second)))
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()

	<-started

	start := time.Now()
	require.NoError(t, r.Stop(context.Background()))
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 500*time.Millisecond, "Stop returned long after drain completed")

	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Run did not return")
	}
}

func TestDraining_TimerForcesCancelWhenWorkIgnoresStopping(t *testing.T) {
	started := make(chan struct{})
	workErr := make(chan error, 1)

	work := func(ctx context.Context) error {
		close(started)
		<-ctx.Done() // ignore Stopping; wait for force-cancel
		workErr <- ctx.Err()
		return ctx.Err()
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(100*time.Millisecond)))
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()

	<-started
	require.NoError(t, r.Stop(context.Background()))

	select {
	case e := <-runErr:
		require.ErrorIs(t, e, adapters.ErrDrainTimedOut)
	case <-time.After(time.Second):
		t.Fatal("Run did not return")
	}
	select {
	case e := <-workErr:
		require.ErrorIs(t, e, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("work did not exit via ctx.Done()")
	}
}

func TestDraining_OuterCtxCancelTriggersDrain(t *testing.T) {
	// Same as previous but cancellation comes from outer ctx, not Stop.
	started := make(chan struct{})

	work := func(ctx context.Context) error {
		close(started)
		select {
		case <-adapters.Stopping(ctx):
			return nil
		case <-ctx.Done():
			return errors.New("ctx cancelled before Stopping")
		}
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(1*time.Second)))
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx) }()

	<-started
	cancel()

	select {
	case err := <-runErr:
		require.NoError(t, err, "work should observe Stopping and exit cleanly")
	case <-time.After(time.Second):
		t.Fatal("Run did not return after outer ctx cancel")
	}
}

func TestDraining_ConcurrentStopsPreserveDrainSemantics(t *testing.T) {
	started := make(chan struct{})
	drainObserved := make(chan struct{})
	var ctxCancelObserved atomic.Bool

	work := func(ctx context.Context) error {
		close(started)
		select {
		case <-adapters.Stopping(ctx):
			close(drainObserved)
			return nil
		case <-ctx.Done():
			ctxCancelObserved.Store(true)
			return ctx.Err()
		}
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(2*time.Second)))
	go func() { _ = r.Run(context.Background()) }()

	<-started

	const callers = 10
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = r.Stop(context.Background())
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			require.ErrorIs(t, err, runnable.ErrNotRunning)
		}
	}

	select {
	case <-drainObserved:
	default:
		t.Fatal("work never observed Stopping(ctx); concurrent Stop bypassed drain")
	}
	assert.False(t, ctxCancelObserved.Load(), "drain bypassed: work saw ctx.Done()")
}

func TestDraining_WorkErrorPropagatesWithoutDrain(t *testing.T) {
	sentinel := errors.New("work failed")
	work := func(ctx context.Context) error { return sentinel }

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(1*time.Second)))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, sentinel)
}

func TestDraining_PublishesDrainStarted(t *testing.T) {
	pub := &capturingPublisher{}
	started := make(chan struct{})

	work := func(ctx context.Context) error {
		close(started)
		<-adapters.Stopping(ctx)
		return nil
	}

	r := runnable.New(work,
		runnable.WithPublisher(pub),
		runnable.WithAdapters(adapters.Draining(1*time.Second)),
	)
	go func() { _ = r.Run(context.Background()) }()

	<-started
	require.NoError(t, r.Stop(context.Background()))

	events := pub.snapshot()
	require.Len(t, events, 1)
	ev, ok := events[0].(runnable.DrainStartedEvent)
	require.True(t, ok, "expected DrainStartedEvent, got %T", events[0])
	assert.Equal(t, time.Second, ev.Timeout)
}

func TestDraining_PublishesDrainTimedOut(t *testing.T) {
	pub := &capturingPublisher{}
	started := make(chan struct{})

	work := func(ctx context.Context) error {
		close(started)
		<-ctx.Done() // ignore Stopping; force the timer to fire
		return ctx.Err()
	}

	r := runnable.New(work,
		runnable.WithPublisher(pub),
		runnable.WithAdapters(adapters.Draining(50*time.Millisecond)),
	)
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()

	<-started
	require.NoError(t, r.Stop(context.Background()))
	require.ErrorIs(t, <-runErr, adapters.ErrDrainTimedOut)

	events := pub.snapshot()
	require.Len(t, events, 2)
	_, ok := events[0].(runnable.DrainStartedEvent)
	require.True(t, ok, "first event should be DrainStartedEvent, got %T", events[0])
	_, ok = events[1].(runnable.DrainTimedOutEvent)
	require.True(t, ok, "second event should be DrainTimedOutEvent, got %T", events[1])
}

func TestDraining_RecoversPanicAsError(t *testing.T) {
	// Regression: panics in work run on Draining's spawned goroutine,
	// not on the goroutine where outer recover defers live. Without
	// internal recovery, a tick panic would crash the process.
	work := func(ctx context.Context) error {
		panic("boom")
	}

	r := runnable.New(work, runnable.WithAdapters(adapters.Draining(1*time.Second)))
	err := r.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom", "panic value should be embedded in error")
	assert.Contains(t, err.Error(), "panic in draining work", "error should identify itself as a recovered panic")
}

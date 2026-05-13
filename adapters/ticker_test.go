package adapters_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xsequence/runnable"
	"github.com/0xsequence/runnable/adapters"
)

func TestTicker_FiresOnInterval(t *testing.T) {
	// Count tick signals on a channel rather than asserting wall-clock
	// arithmetic; loaded CI runners would otherwise queue extra ticks
	// and bust an upper bound. The behavioral claim is "Ticker fires
	// repeatedly on interval" — wait for N ticks, stop, done.
	ticks := make(chan struct{}, 8)
	tick := func(ctx context.Context) error {
		select {
		case ticks <- struct{}{}:
		default:
		}
		return nil
	}

	r := runnable.New(tick, runnable.WithAdapters(adapters.Ticker(20*time.Millisecond)))
	go func() { _ = r.Run(context.Background()) }()

	for i := 0; i < 3; i++ {
		select {
		case <-ticks:
		case <-time.After(time.Second):
			t.Fatalf("only %d ticks observed before timeout", i)
		}
	}
	require.NoError(t, r.Stop(context.Background()))
}

func TestTicker_ComposesWithDraining(t *testing.T) {
	tickStarted := make(chan struct{}, 1)
	var completed atomic.Int32

	tick := func(ctx context.Context) error {
		select {
		case tickStarted <- struct{}{}:
		default:
		}
		time.Sleep(200 * time.Millisecond)
		completed.Add(1)
		return nil
	}

	r := runnable.New(tick, runnable.WithAdapters(
		adapters.Draining(1*time.Second),
		adapters.Ticker(20*time.Millisecond),
	))
	go func() { _ = r.Run(context.Background()) }()

	<-tickStarted

	start := time.Now()
	require.NoError(t, r.Stop(context.Background()))
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, completed.Load(), int32(1), "in-flight tick should complete")
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestTicker_WithoutDrainCancelsInFlightTick(t *testing.T) {
	tickStarted := make(chan struct{}, 1)
	tickErr := make(chan error, 1)

	tick := func(ctx context.Context) error {
		select {
		case tickStarted <- struct{}{}:
		default:
		}
		<-ctx.Done()
		tickErr <- ctx.Err()
		return ctx.Err()
	}

	r := runnable.New(tick, runnable.WithAdapters(adapters.Ticker(20*time.Millisecond)))
	go func() { _ = r.Run(context.Background()) }()

	<-tickStarted
	require.NoError(t, r.Stop(context.Background()))

	select {
	case e := <-tickErr:
		require.ErrorIs(t, e, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("tick did not observe ctx cancellation")
	}
}

func TestTicker_TickErrorAbortsLoop(t *testing.T) {
	sentinel := errors.New("boom")
	var count atomic.Int32

	tick := func(ctx context.Context) error {
		if count.Add(1) == 2 {
			return sentinel
		}
		return nil
	}

	r := runnable.New(tick, runnable.WithAdapters(adapters.Ticker(20*time.Millisecond)))
	err := r.Run(context.Background())
	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, int32(2), count.Load())
}

func TestTicker_RespectsOuterCtxCancel(t *testing.T) {
	var count atomic.Int32
	tick := func(ctx context.Context) error {
		count.Add(1)
		return nil
	}

	r := runnable.New(tick, runnable.WithAdapters(adapters.Ticker(20*time.Millisecond)))
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	err := r.Run(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

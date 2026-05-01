package runnable

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var (
	ErrAlreadyRunning = fmt.Errorf("already running")
	ErrNotRunning     = fmt.Errorf("not running")
	ErrDrainTimedOut  = fmt.Errorf("drain timed out")
)

type Option interface {
	apply(*runnable)
}

type Runnable interface {
	Run(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
}

type runnable struct {
	runFunc func(ctx context.Context) error

	runCtx    context.Context
	runCancel context.CancelFunc
	runStop   chan bool

	drainEnabled bool
	drainTimeout time.Duration
	stoppingChan chan struct{}

	isRunning bool
	onStart   func()
	onStop    func()

	mu sync.Mutex
}

// New creates a new Runnable with the given runFunc.
//
// Example:
//
//	type Monitor struct {
//		runnable.Runnable
//	}
//
//	func (m *Monitor) run(ctx context.Context) error {
//		// do something
//		return nil
//	}
//
//	func NewMonitor() Monitor {
//		m := Monitor{}
//		m.Runnable = runnable.NewRunnable(m.run)
//		return m
//	}
func New(runFunc func(ctx context.Context) error, options ...Option) Runnable {
	r := &runnable{
		runFunc: runFunc,
	}

	for _, option := range options {
		option.apply(r)
	}

	return r
}

// Run starts the runnable, if it is not already running. If the runnable is already running,
// it will return an ErrAlreadyRunning error. If the context is cancelled, it will return the
// context error.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	if err := runnable.Run(ctx); err != nil {
//		log.Error(err)
//	}
func (r *runnable) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return ErrAlreadyRunning
	}

	r.isRunning = true
	r.runCtx, r.runCancel = context.WithCancel(ctx)
	r.runStop = make(chan bool)

	runCtx := r.runCtx
	if r.drainEnabled {
		r.stoppingChan = make(chan struct{})
		runCtx = context.WithValue(runCtx, stoppingKey{}, (<-chan struct{})(r.stoppingChan))
	}
	r.mu.Unlock()

	defer func() {
		if r.onStop != nil {
			r.onStop()
		}

		r.mu.Lock()
		r.isRunning = false
		close(r.runStop)
		r.mu.Unlock()
	}()

	if r.onStart != nil {
		r.onStart()
	}
	return r.runFunc(runCtx)
}

// Stop stops the runnable, if it is running. If the context is cancelled, it will return the context error.
// If the runnable is not running, it will return an error.
// If the runnable is running, it will wait for the runnable to stop before returning.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	if err := runnable.Stop(ctx); err != nil {
//		log.Error(err)
//	}
func (r *runnable) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if !r.isRunning {
		r.mu.Unlock()
		return ErrNotRunning
	}

	runStop := r.runStop
	drainEnabled := r.drainEnabled
	drainTimeout := r.drainTimeout
	stoppingChan := r.stoppingChan
	r.stoppingChan = nil // first-caller wins; subsequent concurrent Stops see nil
	r.mu.Unlock()

	var drainTimedOut bool
	if drainEnabled && stoppingChan != nil {
		close(stoppingChan)
		// Use a standalone timer so the drain budget is independent of
		// the caller's ctx — otherwise a caller ctx shorter than
		// drainTimeout makes <-ctx.Done() and the drain expiry race in
		// the same select.
		drainTimer := time.NewTimer(drainTimeout)
		select {
		case <-runStop:
			drainTimer.Stop()
			return nil
		case <-drainTimer.C:
			drainTimedOut = true
		case <-ctx.Done():
			drainTimer.Stop()
			// Caller deadline elapsed during drain; fall through so
			// r.runCancel() still fires before we return ctx.Err().
		}
	}

	r.runCancel()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-runStop:
		if drainTimedOut {
			return ErrDrainTimedOut
		}
		return nil
	}
}

// IsRunning returns true if the runnable is running, false otherwise.
func (r *runnable) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRunning
}

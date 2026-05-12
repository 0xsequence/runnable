# runnable

![Build & Unit Tests](https://github.com/0xsequence/runnable/actions/workflows/go.yml/badge.svg)


## Overview

`runnable` is a Go package that provides a `Runnable` interface for functions or objects that can be started and stopped.
It provides a simple way to run a function or object in a goroutine and stop it when needed. It also provides a way to run a
function with retry and statistics number of restarts, when started and stopped, if returned error, etc.

## Examples

### Runnable Function
```go
fmt.Println("Simple function...")
err := runnable.New(func(ctx context.Context) error {
    fmt.Println("Starting...")
    defer fmt.Println("Stopping...")

    for i := 0; i < 5; i++ {
        select {
        case <-ctx.Done():
            return nil
        default:
        }
        time.Sleep(1 * time.Second)
        fmt.Println("Running...")
    }
    return nil
}).Run(context.Background())
if err != nil {
    fmt.Println(err)
}
```

### Runnable Function with Stop
```go
fmt.Println("Simple function with stop...")
r := runnable.New(func(ctx context.Context) error {
    fmt.Println("Starting...")
    defer fmt.Println("Stopping...")

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-time.After(time.Second):
        }
        fmt.Println("Running...")
    }
})

go func() {
    time.Sleep(5 * time.Second)

    fmt.Println("Calling Stop...")
    err := r.Stop(context.Background())
    if err != nil {
        fmt.Println(err)
    }
}()

err = r.Run(context.Background())
if err != nil {
    fmt.Println(err)
}
```

### Runnable Function with timeout
```go
fmt.Println("Simple function with timeout...")
ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
err = runnable.New(func(ctx context.Context) error {
    fmt.Println("Starting...")
    defer fmt.Println("Stopping...")

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-time.After(time.Second):
        }
        fmt.Println("Running...")
    }
}).Run(ctxWithTimeout)
if err != nil {
    fmt.Println(err)
}
```

### Runnable Function with retry
```go
fmt.Println("Simple function with retry...")
errorReturned := false
err = runnable.New(func(ctx context.Context) error {
    fmt.Println("Starting...")
    defer fmt.Println("Stopping...")
    
    if !errorReturned {
        errorReturned = true
        return fmt.Errorf("error")
    }
    
    // do something
    for i := 0; i < 5; i++ {
        select {
        case <-ctx.Done():
            return nil
        default:
        }
        time.Sleep(1 * time.Second)
        fmt.Println("Running...")
    }
    return nil
}, runnable.WithRetry(3, runnable.ResetNever)).Run(context.Background())
if err != nil {
    fmt.Println(err)
}
```

### Adapters

Cross-cutting behaviors that aren't part of the core lifecycle live in
the `runnable/adapters` subpackage as runFunc wrappers. They compose by
nesting.

**Ticker** — periodic execution:

```go
r := runnable.New(adapters.Ticker(30*time.Second, reconcile))
```

**Draining** — graceful shutdown with a grace window. When the outer
ctx is cancelled, work has `timeout` to return via `adapters.Stopping(ctx)`
before its ctx is force-cancelled and `adapters.ErrDrainTimedOut` is
returned.

```go
r := runnable.New(adapters.Draining(10*time.Second,
    adapters.Ticker(30*time.Second, reconcile),
))
```

Inside long-running work, always select on both `ctx.Done()` and
`adapters.Stopping(ctx)` — `Stopping` signals drain start, `ctx.Done()`
fires only when the drain timer expires.

A full SIGTERM-safe service shape lives in
[`examples/ticker-with-drain`](examples/ticker-with-drain/main.go).

### Migrating from v0.1 to v0.2

v0.2 moves drain and ticker out of the core package. The Option-based
`WithDrain` and the `NewTicker` constructor are removed; their
replacements live at `runnable/adapters`.

Before (v0.1):

    r := runnable.NewTicker(30*time.Second, doWork,
        runnable.WithDrain(10*time.Second),
    )

After (v0.2):

    r := runnable.New(adapters.Draining(10*time.Second,
        adapters.Ticker(30*time.Second, doWork),
    ))

Symbol mapping:

- `runnable.WithDrain` → use `adapters.Draining` as a runFunc wrapper.
- `runnable.NewTicker` → `adapters.Ticker` wrapped by `runnable.New`.
- `runnable.Stopping` → `adapters.Stopping`.
- `runnable.ErrDrainTimedOut` → `adapters.ErrDrainTimedOut`.

**Behavioral change:** `Stop(ctx)`'s ctx no longer shortens the drain
window. In v0.1, a caller ctx shorter than `WithDrain`'s timeout would
force-cancel mid-drain. In v0.2, `Stop`'s ctx only governs how long
the caller waits for `Stop` to return; the drain runs on its own
fixed-duration timer regardless. If you need a shorter drain budget,
configure `Draining` with the shorter duration.

**NewGroup interaction:** drain-enabled children of `NewGroup` now
drain when the group is stopped (v0.1 silently bypassed the child's
drain). No code change required at call sites — the adapter design
fixes this by construction.

### Runnable Object
```go
package main

import (
	"time"

	"github.com/0xsequence/runnable"
)

type Monitor struct {
	runnable.Runnable
}

func NewMonitor() *Monitor {
	m := &Monitor{}
	m.Runnable = runnable.New(m.run)
	return m
}

func (m *Monitor) run(ctx context.Context) error {
	fmt.Println("Starting...")
	defer fmt.Println("Stopping...")
	
	// Start monitoring
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		time.Sleep(1 * time.Second)
		fmt.Println("Monitoring...")
	}
	return nil
}

func main() {
	fmt.Println("Runnable object(Monitor)...")
	m := NewMonitor()

	go func() {
		time.Sleep(5 * time.Second)

		fmt.Println("Calling Stop...")
		err := m.Stop(context.Background())
		if err != nil {
			fmt.Println(err)
		}
	}()

	err = m.Run(context.Background())
	if err != nil {
		fmt.Println(err)
	}
}
```

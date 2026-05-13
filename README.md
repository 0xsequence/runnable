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

### Adapters

Cross-cutting behaviors that aren't part of the core lifecycle live in
the `runnable/adapters` subpackage as chi-style middleware: each
`runnable.Adapter` has the shape `func(next RunFunc) RunFunc`. Apply
them with `runnable.WithAdapters` (left-to-right = outermost-to-innermost):

```go
r := runnable.New(reconcile, runnable.WithAdapters(
    adapters.Draining(10*time.Second),
    adapters.Recovering(reportPanic),
    adapters.Retry(3, time.Minute),
    adapters.Ticker(30*time.Second),
))
```

**Draining** — graceful shutdown with a grace window. When the outer
ctx is cancelled, the wrapped work has `timeout` to return via
`adapters.Stopping(ctx)` before its ctx is force-cancelled and
`adapters.ErrDrainTimedOut` is returned.

**Ticker** — calls the wrapped work once per interval until ctx is
cancelled or the work returns an error. Composes with Draining: an
in-flight tick is allowed to finish before the loop exits.

**Recovering** — turns panics in the wrapped work into errors and
invokes the optional handler before returning. Place inside Draining
when both are in use.

**Retry** — re-invokes the wrapped work up to `maxRetries` times on
non-context errors. If `resetAfter` is non-zero and at least that long
has passed since the previous attempt, the retry budget resets.

Inside long-running work, always select on both `ctx.Done()` and
`adapters.Stopping(ctx)` — `Stopping` signals drain start, `ctx.Done()`
fires only when the drain timer expires.

A full SIGTERM-safe service shape lives in
[`examples/ticker-with-drain`](examples/ticker-with-drain/main.go).

### Migrating from v0.1 to v0.2

v0.2 moves drain, ticker, retry, and panic recovery out of the core
package. `WithDrain`, `NewTicker`, `WithRetry`, and `WithRecoverer`
are removed; their replacements live at `runnable/adapters` as
chi-style middleware.

Before (v0.1):

    r := runnable.NewTicker(30*time.Second, doWork,
        runnable.WithDrain(10*time.Second),
        runnable.WithRecoverer(reporter, nil),
        runnable.WithRetry(3, time.Minute),
    )

After (v0.2):

    r := runnable.New(doWork, runnable.WithAdapters(
        adapters.Draining(10*time.Second),
        adapters.Recovering(handler),
        adapters.Retry(3, time.Minute),
        adapters.Ticker(30*time.Second),
    ))

Symbol mapping:

- `runnable.WithDrain` → `adapters.Draining` under `runnable.WithAdapters`.
- `runnable.NewTicker` → `adapters.Ticker` under `runnable.WithAdapters`
  (no longer takes the work argument; pass work to `runnable.New`).
- `runnable.WithRetry` / `runnable.ResetNever` → `adapters.Retry` /
  `adapters.ResetNever`.
- `runnable.WithRecoverer` → `adapters.Recovering` with a single
  `PanicHandler` callback (the two-interface `RecoveryReporter` /
  `StackPrinter` split is gone).
- `runnable.Stopping` → `adapters.Stopping`.
- `runnable.ErrDrainTimedOut` → `adapters.ErrDrainTimedOut`.

**Behavioral change:** `Stop(ctx)`'s ctx no longer shortens the drain
window. In v0.1, a caller ctx shorter than `WithDrain`'s timeout would
force-cancel mid-drain. In v0.2, `Stop`'s ctx only governs how long
the caller waits for `Stop` to return; the drain runs on its own
fixed-duration timer regardless. If you need a shorter drain budget,
configure `Draining` with the shorter duration.

**Status.Restarts removed.** The `Restarts` field on `Status` counted
`WithRetry` re-entries via the deprecated `onStart` coupling; with
retry moved into adapters it had no clean way to surface. Pending a
proper event/observer hook in a later release.

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

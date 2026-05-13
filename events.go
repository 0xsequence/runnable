package runnable

import "time"

// RetryEvent is published by adapters.Retry after a failed attempt
// (before sleeping or retrying). Attempt is 1-indexed.
type RetryEvent struct {
	Attempt int
	Err     error
}

// DrainStartedEvent is published by adapters.Draining when the outer
// ctx is cancelled and the drain window begins.
type DrainStartedEvent struct {
	Timeout time.Duration
}

// DrainTimedOutEvent is published by adapters.Draining when the drain
// window expires and work is force-cancelled.
type DrainTimedOutEvent struct{}

// PanicRecoveredEvent is published by adapters.Recovering when it
// catches a panic from the wrapped work.
type PanicRecoveredEvent struct {
	Recovered any
	Stack     []byte
}

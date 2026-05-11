// Package adapters provides composable wrappers around the runFunc
// signature (func(context.Context) error). Adapters layer cross-cutting
// behavior such as drain-on-shutdown (Draining) and periodic execution
// (Ticker) without coupling those concerns into the core runnable
// lifecycle.
//
// Adapters compose by nesting:
//
//	r := runnable.New(adapters.Draining(10*time.Second,
//	    adapters.Ticker(time.Second, doWork),
//	))
package adapters

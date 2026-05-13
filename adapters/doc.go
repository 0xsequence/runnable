// Package adapters provides chi-style middleware around the runnable
// RunFunc signature. Each constructor returns a runnable.Adapter;
// compose them via runnable.WithAdapters (first listed = outermost):
//
//	r := runnable.New(reconcile, runnable.WithAdapters(
//	    adapters.Draining(10*time.Second),
//	    adapters.Ticker(time.Second),
//	))
package adapters

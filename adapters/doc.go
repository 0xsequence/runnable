// Package adapters provides chi-style middleware around the runnable
// RunFunc signature. Each adapter is a config-only constructor that
// returns a runnable.Adapter; compose them via runnable.WithAdapters
// (first listed = outermost wrapper):
//
//	r := runnable.New(reconcile, runnable.WithAdapters(
//	    adapters.Draining(10*time.Second),
//	    adapters.Recovering(reportPanic),
//	    adapters.Retry(3, time.Minute),
//	    adapters.Ticker(time.Second),
//	))
//
// Drain-on-shutdown (Draining), panic-to-error conversion (Recovering),
// retry-on-error (Retry), and periodic execution (Ticker) all live here
// rather than in the core runnable package so they don't couple into
// the core lifecycle.
package adapters

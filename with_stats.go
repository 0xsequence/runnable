package runnable

import (
	"context"
	"sync"
	"time"
)

type StatsView struct {
	Runnable map[string]RunnableStatsView `json:"runnable"`
}

type RunnableStatsView struct {
	Running   bool       `json:"running"`
	Restarts  int        `json:"restarts"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	LastError error      `json:"last_error"`
}

type Stats struct {
	running   map[string]bool
	restarts  map[string]int
	startTime map[string]time.Time
	endTime   map[string]time.Time
	lastError map[string]error

	mu sync.Mutex
}

func NewStats() *Stats {
	return &Stats{
		running:   make(map[string]bool),
		restarts:  make(map[string]int),
		startTime: make(map[string]time.Time),
		endTime:   make(map[string]time.Time),
		lastError: make(map[string]error),
	}
}

func (s *Stats) Get() StatsView {
	s.mu.Lock()
	defer s.mu.Unlock()

	svs := StatsView{Runnable: map[string]RunnableStatsView{}}

	for id, running := range s.running {
		sv := RunnableStatsView{
			Running: running,
		}

		if restarts, ok := s.restarts[id]; ok {
			sv.Restarts = restarts
		}

		if startTime, ok := s.startTime[id]; ok {
			sv.StartTime = startTime
		}

		if endTime, ok := s.endTime[id]; ok {
			et := endTime
			sv.EndTime = &et
		}

		if lastError, ok := s.lastError[id]; ok {
			sv.LastError = lastError
		}

		svs.Runnable[id] = sv
	}

	return svs
}

type withStats struct {
	id    string
	stats *Stats
}

func (w *withStats) apply(r *runnable) {
	runFuncRunnable := r.runFunc
	onStartRunnable := r.onStart
	onStopRunnable := r.onStop

	r.runFunc = func(ctx context.Context) error {
		defer func() {
			w.stats.mu.Lock()
			w.stats.running[w.id] = false
			w.stats.endTime[w.id] = time.Now()
			w.stats.mu.Unlock()
		}()

		err := runFuncRunnable(ctx)
		if err != nil {
			w.stats.mu.Lock()
			w.stats.lastError[w.id] = err
			w.stats.mu.Unlock()
			return err
		}
		return nil
	}

	r.onStart = func() {
		w.stats.mu.Lock()

		w.stats.running[w.id] = true
		w.stats.startTime[w.id] = time.Now()
		if _, ok := w.stats.restarts[w.id]; !ok {
			w.stats.restarts[w.id] = 0
		} else {
			w.stats.restarts[w.id]++
		}

		w.stats.mu.Unlock()

		if onStartRunnable != nil {
			onStartRunnable()
		}
	}

	r.onStop = func() {
		w.stats.mu.Lock()
		w.stats.running[w.id] = false
		w.stats.endTime[w.id] = time.Now()
		w.stats.mu.Unlock()

		if onStopRunnable != nil {
			onStopRunnable()
		}
	}
}

func WithStats(id string, stats *Stats) Option {
	return &withStats{
		id:    id,
		stats: stats,
	}
}

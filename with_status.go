package runnable

import (
	"context"
	"sync"
	"time"
)

type StatusMap map[string]Status

type Status struct {
	Running   bool       `json:"running"`
	Restarts  int        `json:"restarts"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	LastError error      `json:"last_error"`
}

type StatusStore struct {
	running   map[string]bool
	restarts  map[string]int
	startTime map[string]time.Time
	endTime   map[string]time.Time
	lastError map[string]error

	mu sync.Mutex
}

func NewStatusStore() *StatusStore {
	return &StatusStore{
		running:   make(map[string]bool),
		restarts:  make(map[string]int),
		startTime: make(map[string]time.Time),
		endTime:   make(map[string]time.Time),
		lastError: make(map[string]error),
	}
}

func (s *StatusStore) Get() StatusMap {
	s.mu.Lock()
	defer s.mu.Unlock()

	sm := StatusMap{}
	for id, running := range s.running {
		st := Status{
			Running: running,
		}

		if restarts, ok := s.restarts[id]; ok {
			st.Restarts = restarts
		}

		if startTime, ok := s.startTime[id]; ok {
			st.StartTime = startTime
		}

		if endTime, ok := s.endTime[id]; ok {
			et := endTime
			st.EndTime = &et
		}

		if lastError, ok := s.lastError[id]; ok {
			st.LastError = lastError
		}

		sm[id] = st
	}

	return sm
}

type withStatus struct {
	runnableID string
	store      *StatusStore
}

func (w *withStatus) apply(r *runnable) {
	runFuncRunnable := r.runFunc
	onStartRunnable := r.onStart
	onStopRunnable := r.onStop

	r.runFunc = func(ctx context.Context) error {
		defer func() {
			w.store.mu.Lock()
			w.store.running[w.runnableID] = false
			w.store.endTime[w.runnableID] = time.Now()
			w.store.mu.Unlock()
		}()

		err := runFuncRunnable(ctx)
		if err != nil {
			w.store.mu.Lock()
			w.store.lastError[w.runnableID] = err
			w.store.mu.Unlock()
			return err
		}
		return nil
	}

	r.onStart = func() {
		w.store.mu.Lock()

		w.store.running[w.runnableID] = true
		w.store.startTime[w.runnableID] = time.Now()
		if _, ok := w.store.restarts[w.runnableID]; !ok {
			w.store.restarts[w.runnableID] = 0
		} else {
			w.store.restarts[w.runnableID]++
		}

		w.store.mu.Unlock()

		if onStartRunnable != nil {
			onStartRunnable()
		}
	}

	r.onStop = func() {
		w.store.mu.Lock()
		w.store.running[w.runnableID] = false
		w.store.endTime[w.runnableID] = time.Now()
		w.store.mu.Unlock()

		if onStopRunnable != nil {
			onStopRunnable()
		}
	}
}

func WithStatus(id string, store *StatusStore) Option {
	return &withStatus{
		runnableID: id,
		store:      store,
	}
}

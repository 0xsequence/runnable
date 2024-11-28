package runnable

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type InMemoryReporter struct {
	logs []string
}

func (i *InMemoryReporter) Report(ctx context.Context, rec interface{}) {
	i.logs = append(i.logs, fmt.Sprintf("%s", rec.(string)))
}

func TestWithRecoverer(t *testing.T) {
	t.Run("with recoverer", func(t *testing.T) {
		counter := 0
		reporter := InMemoryReporter{}

		fn := func(ctx context.Context) error {
			defer func() { counter++ }()
			panic("something went wrong")
			return nil
		}
		r := New(fn, WithRecoverer(&reporter, nil))

		err := r.Run(context.Background())
		require.Error(t, err)
		assert.Equal(t, 1, counter)
		assert.Equal(t, []string{"something went wrong"}, reporter.logs)
	})

	t.Run("panics as errors", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})
		reporter := &InMemoryReporter{}

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			panic("something went wrong")
			return nil
		}, WithRecoverer(reporter, nil))

		go func() {
			err := r.Run(context.Background())
			require.Error(t, err)
			stopped <- struct{}{}
		}()

		<-started
		<-stopped
	})

	t.Run("panics as errors, no panic", func(t *testing.T) {
		reporter := &InMemoryReporter{}
		started, stopped := make(chan struct{}), make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			return nil
		}, WithRecoverer(reporter, nil))

		go func() {
			err := r.Run(context.Background())
			require.NoError(t, err)
			stopped <- struct{}{}
		}()

		<-started
		<-stopped
	})

	t.Run("panics as errors, with stats", func(t *testing.T) {
		reporter := &InMemoryReporter{}
		started, stopped := make(chan struct{}), make(chan struct{})

		store := NewStatusStore()
		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			panic("something went wrong")
			return nil
		}, WithRecoverer(reporter, nil), WithStatus("test", store))

		go func() {
			err := r.Run(context.Background())
			require.Error(t, err)
			stopped <- struct{}{}
		}()

		<-started
		<-stopped

		s := store.Get()
		require.Equal(t, false, s["test"].Running)
		require.Error(t, s["test"].LastError)
	})
}

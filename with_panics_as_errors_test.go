package runnable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithPanicsAsErrors(t *testing.T) {

	t.Run("panics as errors", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			panic("something went wrong")
			return nil
		}, WithPanicsAsErrors())

		go func() {
			err := r.Run(context.Background())
			require.Error(t, err)
			stopped <- struct{}{}
		}()

		<-started
		<-stopped
	})

	t.Run("panics as errors, no panic", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})

		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			return nil
		}, WithPanicsAsErrors())

		go func() {
			err := r.Run(context.Background())
			require.NoError(t, err)
			stopped <- struct{}{}
		}()

		<-started
		<-stopped
	})

	t.Run("panics as errors, with stats", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})

		store := NewStatusStore()
		r := New(func(ctx context.Context) error {
			started <- struct{}{}
			panic("something went wrong")
			return nil
		}, WithPanicsAsErrors(), WithStatus("test", store))

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

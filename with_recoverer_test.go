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

func (i *InMemoryReporter) Report(ctx context.Context, runnableId string, rec interface{}) {
	i.logs = append(i.logs, fmt.Sprintf("%s - %s", runnableId, rec.(string)))
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
		r := New(fn, WithRecoverer("datasync", &reporter))

		err := r.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, counter)
		assert.Equal(t, []string{"datasync - something went wrong"}, reporter.logs)
	})
}

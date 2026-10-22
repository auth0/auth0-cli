package filelock_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/filelock"
)

// errBusy stands for the error that a caller supplies for a lock that stays held.
var errBusy = errors.New("busy")

func TestAcquire(t *testing.T) {
	t.Run("creates the lock file with restricted permissions", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".lock")

		release, err := filelock.Acquire(context.Background(), path, errBusy)
		require.NoError(t, err)
		defer release()

		info, err := os.Stat(path)
		require.NoError(t, err)

		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	})

	t.Run("fails when the lock file cannot be created", func(t *testing.T) {
		_, err := filelock.Acquire(context.Background(), filepath.Join(t.TempDir(), "missing", ".lock"), errBusy)

		require.Error(t, err)
		assert.NotErrorIs(t, err, errBusy)
	})

	t.Run("gives up with ErrBusy while the lock is held, and wraps the error of the context", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".lock")

		release, err := filelock.Acquire(context.Background(), path, errBusy)
		require.NoError(t, err)
		defer release()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = filelock.Acquire(ctx, path, errBusy)

		require.ErrorIs(t, err, errBusy)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("reports a context that is already cancelled", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".lock")

		release, err := filelock.Acquire(context.Background(), path, errBusy)
		require.NoError(t, err)
		defer release()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = filelock.Acquire(ctx, path, errBusy)

		require.ErrorIs(t, err, errBusy)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("can be taken again once it was released", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".lock")

		release, err := filelock.Acquire(context.Background(), path, errBusy)
		require.NoError(t, err)
		release()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		again, err := filelock.Acquire(ctx, path, errBusy)
		require.NoError(t, err)
		again()
	})

	t.Run("lets only one holder in at a time", func(t *testing.T) {
		const holders = 8

		path := filepath.Join(t.TempDir(), ".lock")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// The most holders that were inside at the same time.
		var inside, most atomic.Int32

		var wg sync.WaitGroup

		errs := make([]error, holders)

		for n := range holders {
			wg.Add(1)

			go func() {
				defer wg.Done()

				release, err := filelock.Acquire(ctx, path, errBusy)
				if err != nil {
					errs[n] = err

					return
				}
				defer release()

				now := inside.Add(1)
				most.Store(max(most.Load(), now))
				time.Sleep(5 * time.Millisecond)
				inside.Add(-1)
			}()
		}

		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		assert.EqualValues(t, 1, most.Load(), "two holders were inside at the same time")
	})
}

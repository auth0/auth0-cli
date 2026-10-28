package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "access denied", err: errorAccessDenied, want: true},
		{name: "sharing violation", err: errorSharingViolation, want: true},
		{name: "lock violation", err: errorLockViolation, want: true},
		{name: "as os.Rename reports it", err: &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errorAccessDenied}, want: true},
		{name: "wrapped by a caller", err: fmt.Errorf("failed to move: %w", &os.LinkError{Op: "rename", Err: errorSharingViolation}), want: true},
		{name: "a file that does not exist", err: &os.LinkError{Op: "rename", Err: syscall.ENOENT}},
		{name: "a plain error", err: errors.New("disk is read only")},
		{name: "no error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isRetryable(test.err))
		})
	}
}

func TestRenameDelay(t *testing.T) {
	var got []time.Duration

	for attempt := range renameTries - 1 {
		got = append(got, renameDelay(attempt))
	}

	want := []time.Duration{
		20 * time.Millisecond, 40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond,
		200 * time.Millisecond, 200 * time.Millisecond, 200 * time.Millisecond, 200 * time.Millisecond, 200 * time.Millisecond,
	}

	assert.Equal(t, want, got)
	assert.Equal(t, 200*time.Millisecond, renameDelay(1000), "a large attempt number stays at the cap")
}

func TestRetryRename(t *testing.T) {
	busy := &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errorAccessDenied}

	tests := []struct {
		name string

		// Failures is how many calls fail with err before one succeeds. A negative
		// number means every call fails.
		failures  int
		err       error
		wantCalls int
		wantWaits int
		wantErr   error
	}{
		{name: "works at once", failures: 0, err: busy, wantCalls: 1, wantWaits: 0},
		{name: "works after the file is closed", failures: 3, err: busy, wantCalls: 4, wantWaits: 3},
		{name: "works on the last try", failures: renameTries - 1, err: busy, wantCalls: renameTries, wantWaits: renameTries - 1},
		{name: "gives up with the error of the last try", failures: -1, err: busy, wantCalls: renameTries, wantWaits: renameTries - 1, wantErr: busy},
		{name: "does not retry another error", failures: -1, err: fs.ErrNotExist, wantCalls: 1, wantWaits: 0, wantErr: fs.ErrNotExist},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				calls int
				waits []time.Duration
			)

			rename := func(oldpath, newpath string) error {
				calls++

				assert.Equal(t, "from", oldpath)
				assert.Equal(t, "to", newpath)

				if test.failures < 0 || calls <= test.failures {
					return test.err
				}

				return nil
			}

			err := retryRename(rename, func(d time.Duration) { waits = append(waits, d) }, "from", "to")

			if test.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.wantErr)
			}

			assert.Equal(t, test.wantCalls, calls)
			require.Len(t, waits, test.wantWaits)

			for attempt, wait := range waits {
				assert.Equal(t, renameDelay(attempt), wait)
			}
		})
	}
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")

	require.NoError(t, os.WriteFile(from, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(to, []byte("old"), 0o600))

	require.NoError(t, Rename(from, to))

	data, err := os.ReadFile(to)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
	assert.NoFileExists(t, from)

	require.ErrorIs(t, Rename(from, to), fs.ErrNotExist, "a missing source is reported and not retried")
}

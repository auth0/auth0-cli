package plugins

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestRetryableCodesAreWin32Codes(t *testing.T) {
	assert.Equal(t, windows.ERROR_ACCESS_DENIED, errorAccessDenied)
	assert.Equal(t, windows.ERROR_SHARING_VIOLATION, errorSharingViolation)
	assert.Equal(t, windows.ERROR_LOCK_VIOLATION, errorLockViolation)
}

func TestIsInUse(t *testing.T) {
	for _, code := range []windows.Errno{windows.ERROR_ACCESS_DENIED, windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION} {
		assert.True(t, IsInUse(&os.LinkError{Op: "rename", Err: code}), "%v", code)
	}
}

// The tests hold a file open with os.Open, which does not let the file be
// deleted or replaced while it is open. That is the situation of a virus scanner
// or an indexer that reads a file the CLI wants to replace.

func TestRename_WaitsForAReaderToLetGo(t *testing.T) {
	tests := []struct {
		name string

		// Setup creates what is renamed and returns its path, the destination and
		// the file to hold open.
		setup func(t *testing.T, dir string) (from, to, held string)

		// Check looks at the destination after the rename.
		check func(t *testing.T, to string)
	}{
		{
			name: "a file that is replaced while it is open",
			setup: func(t *testing.T, dir string) (string, string, string) {
				target, staged := filepath.Join(dir, "target"), filepath.Join(dir, "staged")

				require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
				require.NoError(t, os.WriteFile(staged, []byte("new"), 0o600))

				return staged, target, target
			},
			check: func(t *testing.T, to string) {
				data, err := os.ReadFile(to)
				require.NoError(t, err)
				assert.Equal(t, "new", string(data))
			},
		},
		{
			name: "a directory that holds an open file",
			setup: func(t *testing.T, dir string) (string, string, string) {
				from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")

				require.NoError(t, os.Mkdir(from, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(from, "file"), []byte("content"), 0o600))

				return from, to, filepath.Join(from, "file")
			},
			check: func(t *testing.T, to string) {
				assert.FileExists(t, filepath.Join(to, "file"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from, to, held := test.setup(t, t.TempDir())

			holder, err := os.Open(held)
			require.NoError(t, err)

			t.Cleanup(func() { _ = holder.Close() })

			err = os.Rename(from, to)
			require.Error(t, err, "os.Rename alone fails while the file is open")
			assert.True(t, isRetryable(err), "%v", err)

			// The reader lets go after the first attempts have failed.
			time.AfterFunc(100*time.Millisecond, func() { _ = holder.Close() })

			require.NoError(t, Rename(from, to))
			test.check(t, to)
		})
	}
}

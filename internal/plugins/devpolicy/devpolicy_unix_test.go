//go:build !windows

package devpolicy

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A named pipe has no writer, so opening it would wait for ever. The file must be
// refused before it is opened.
func TestLoad_ANamedPipeDoesNotHang(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	require.NoError(t, syscall.Mkfifo(path, 0o600))

	done := make(chan error, 1)

	go func() {
		_, err := Load(path)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalid)
		assert.Contains(t, err.Error(), "not a regular file")
	case <-time.After(10 * time.Second):
		t.Fatal("Load waited on a named pipe")
	}
}

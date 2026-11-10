//go:build !windows

package cli

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPluginHint_NamedPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	require.NoError(t, syscall.Mkfifo(pipe, 0o600))

	done := make(chan bool, 1)

	go func() {
		_, ok := readPluginHint(pipe)
		done <- ok
	}()

	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(10 * time.Second):
		t.Fatal("reading the hint file blocked on a named pipe")
	}
}

//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// processAlive reports whether a process with the ID exists.
func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}

func TestPluginRun_Hangup(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	marker := filepath.Join(t.TempDir(), "pid")
	done := make(chan error, 1)

	go func() { done <- r.exec("checkmate", "--mode=sleep", "--marker="+marker) }()

	pid := waitForMarker(t, marker)

	// A terminal that is closed sends SIGHUP to the CLI. The run has taken the
	// signal by now, as the plugin only starts after that.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))

	err := <-done

	require.Error(t, err)

	class, reason := reasonOf(err)
	assert.Equal(t, "plugin", class)
	assert.Equal(t, "plugin_interrupted", reason)
	assert.False(t, processAlive(pid), "the plugin must be gone")
}

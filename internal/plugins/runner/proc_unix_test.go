//go:build !windows

package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	helperModes["groups"] = func([]string) {
		fmt.Fprintf(os.Stdout, "%d %d", os.Getpid(), syscall.Getpgrp())
	}
	helperModes["kill-self"] = func([]string) {
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)

		time.Sleep(time.Minute)
	}
}

func TestRun_ProcessGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interactive bool
		ownGroup    bool
	}{
		{"a command that is not interactive has a group of its own", false, true},
		{"an interactive command stays in the group of the CLI", true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer

			spec := helper("groups")
			spec.Stdout = &stdout
			spec.Interactive = test.interactive

			require.NoError(t, Run(t.Context(), spec))

			var pid, group int

			_, err := fmt.Sscanf(strings.TrimSpace(stdout.String()), "%d %d", &pid, &group)
			require.NoError(t, err)

			if test.ownGroup {
				assert.Equal(t, pid, group)
			} else {
				assert.Equal(t, syscall.Getpgrp(), group)
			}
		})
	}
}

func TestRun_ASignalIsReportedAs128PlusTheSignal(t *testing.T) {
	t.Parallel()

	err := Run(t.Context(), helper("kill-self"))

	var exit *ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 128+int(syscall.SIGKILL), exit.Code)
}

func TestRun_StoppingAnInteractiveCommandLeavesWhatItStarted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		waitFor(t, filepath.Join(dir, "ready"))
		cancel()
	}()

	spec := helper("spawn", dir)
	spec.Interactive = true

	// The grandchild is not ours to leave behind once the test is done.
	t.Cleanup(func() {
		if data, err := os.ReadFile(filepath.Join(dir, "ready")); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})

	var interrupted *InterruptedError
	require.ErrorAs(t, Run(ctx, spec), &interrupted)

	// Only the plugin is signalled, never the group of the CLI, so what the plugin
	// started keeps running.
	before := beats(t, dir)

	assert.Eventually(t, func() bool { return beats(t, dir) != before }, 5*time.Second, 20*time.Millisecond)
}

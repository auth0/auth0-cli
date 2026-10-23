//go:build windows

package runner

import (
	"os"
	"os/exec"
)

// TODO(DXCDT-2610): Job Objects. Until then a plugin on Windows runs as a plain
// child: a timeout or Ctrl-C ends the plugin process only, and anything it
// started can outlive it.

// prepare does nothing on Windows.
func prepare(*exec.Cmd, bool) {}

// terminate ends the plugin, because Windows has no signal that asks a process to
// stop.
func terminate(cmd *exec.Cmd, _ bool) {
	_ = cmd.Process.Kill()
}

// kill ends the plugin.
func kill(cmd *exec.Cmd, _ bool) {
	_ = cmd.Process.Kill()
}

// lingering is always false on Windows, where only the plugin itself is tracked.
func lingering(*exec.Cmd, bool) bool {
	return false
}

// exitCode returns the exit code of a finished process.
func exitCode(state *os.ProcessState) int {
	return state.ExitCode()
}

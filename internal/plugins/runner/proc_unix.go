//go:build !windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// prepare puts a plugin that is not interactive in a process group of its own,
// which is what lets the runner stop everything it starts. An interactive plugin
// stays in the group of the CLI, because a background group stops on its first
// read from the terminal.
func prepare(cmd *exec.Cmd, interactive bool) {
	if !interactive {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

// terminate asks the plugin, and its group when it has one, to stop.
func terminate(cmd *exec.Cmd, interactive bool) {
	send(cmd, interactive, syscall.SIGTERM)
}

// kill ends the plugin, and its group when it has one, without a chance to clean
// up.
func kill(cmd *exec.Cmd, interactive bool) {
	send(cmd, interactive, syscall.SIGKILL)
}

// send delivers a signal to the group of the plugin, or to the plugin alone when
// it is interactive. A process that is already gone is not an error.
func send(cmd *exec.Cmd, interactive bool, signal syscall.Signal) {
	if interactive {
		_ = cmd.Process.Signal(signal)

		return
	}

	_ = syscall.Kill(-cmd.Process.Pid, signal)
}

// lingering reports whether the group of the plugin still has a process in it,
// after the plugin itself has gone. An interactive plugin has no group of its
// own, so nothing is left to wait for.
func lingering(cmd *exec.Cmd, interactive bool) bool {
	if interactive {
		return false
	}

	err := syscall.Kill(-cmd.Process.Pid, 0)

	return err == nil || errors.Is(err, syscall.EPERM)
}

// exitCode returns the exit code of a finished process, or 128 plus the number
// of the signal that ended it.
func exitCode(state *os.ProcessState) int {
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}

	return state.ExitCode()
}

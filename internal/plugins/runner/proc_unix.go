//go:build !windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// tree is what the runner needs to stop the processes of one run. On Unix that
// is the plugin and, when it is not interactive, its process group.
type tree struct {
	cmd         *exec.Cmd
	interactive bool
}

// prepare puts a plugin that is not interactive in a process group of its own,
// which is what lets the runner stop everything it starts. An interactive plugin
// stays in the group of the CLI, because a background group stops on its first
// read from the terminal.
func prepare(cmd *exec.Cmd, interactive bool) *tree {
	if !interactive {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	return &tree{cmd: cmd, interactive: interactive}
}

// started is called after the plugin has started. Nothing is needed on Unix.
func (*tree) started() error {
	return nil
}

// terminate asks the plugin, and its group when it has one, to stop.
func (t *tree) terminate() {
	t.send(syscall.SIGTERM)
}

// kill ends the plugin, and its group when it has one, without a chance to clean
// up.
func (t *tree) kill() {
	t.send(syscall.SIGKILL)
}

// send delivers a signal to the group of the plugin, or to the plugin alone when
// it is interactive. A process that is already gone is not an error.
func (t *tree) send(signal syscall.Signal) {
	if t.interactive {
		_ = t.cmd.Process.Signal(signal)

		return
	}

	_ = syscall.Kill(-t.cmd.Process.Pid, signal)
}

// lingering reports whether the group of the plugin still has a process in it,
// after the plugin itself has gone. An interactive plugin has no group of its
// own, so nothing is left to wait for.
func (t *tree) lingering() bool {
	if t.interactive {
		return false
	}

	err := syscall.Kill(-t.cmd.Process.Pid, 0)

	return err == nil || errors.Is(err, syscall.EPERM)
}

// release is called when the run is over. There is nothing to free on Unix.
func (*tree) release(bool) {}

// exitCode returns the exit code of a finished process, or 128 plus the number
// of the signal that ended it.
func exitCode(state *os.ProcessState) int {
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}

	return state.ExitCode()
}

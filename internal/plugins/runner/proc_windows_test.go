package runner

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func init() {
	// Leave <dir> starts a process that beats, waits until it is beating, and
	// exits by itself.
	helperModes["leave"] = func(args []string) {
		child := helperChild("beat", args[0])

		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitHelper(3)
		}

		for i := 0; i < 6000; i++ {
			if _, err := os.Stat(filepath.Join(args[0], "ready")); err == nil {
				return
			}

			time.Sleep(5 * time.Millisecond)
		}

		exitHelper(4)
	}

	// Breakaway <dir> tries to start a process outside of its job object, and
	// writes to <dir>/result whether that was refused.
	helperModes["breakaway"] = func(args []string) {
		child := helperChild("sleep")
		child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}

		result := "refused"

		if err := child.Start(); err == nil {
			result = "escaped"

			_ = child.Process.Kill()
		}

		_ = os.WriteFile(filepath.Join(args[0], "result"), []byte(result), 0o600)
	}
}

// ignoreStop makes the process ignore Ctrl-Break, which is the request to stop.
// Go ends a process on Ctrl-Break unless a program asks to be told about it.
func ignoreStop() {
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
}

// terminateBeat ends the process that beats in dir.
func terminateBeat(dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "ready"))
	if err != nil {
		return
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return
	}

	process, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}

	defer windows.CloseHandle(process)

	_ = windows.TerminateProcess(process, 1)
}

func TestRun_CtrlCIsReportedAs130(t *testing.T) {
	t.Parallel()

	err := Run(t.Context(), helper("exit", strconv.FormatUint(controlCExit, 10)))

	var exit *ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 130, exit.Code)
}

func TestRun_ExitingByItselfLeavesWhatItStarted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// The process is not ours to leave behind once the test is done.
	t.Cleanup(func() { terminateBeat(dir) })

	require.NoError(t, Run(t.Context(), helper("leave", dir)))

	before := beats(t, dir)

	assert.Eventually(t, func() bool { return beats(t, dir) != before }, 5*time.Second, 20*time.Millisecond, "the process did not keep running")
}

func TestRun_AProcessCannotLeaveTheJob(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, Run(t.Context(), helper("breakaway", dir)))

	result, err := os.ReadFile(filepath.Join(dir, "result"))
	require.NoError(t, err)

	assert.Equal(t, "refused", string(result))
}

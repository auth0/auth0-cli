//go:build windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ntResumeProcess is the call that resumes every thread of a process. Go closes
// the handle of the first thread right after it creates a process, so a process
// that starts suspended cannot be resumed with ResumeThread.
var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// jobAccounting is JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, which the windows
// package does not define.
type jobAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// The structure must be as large as the one the system fills in, 48 bytes. The
// array types are the same only when the length is, so a structure that is too
// large or too small does not compile.
var _ [48]struct{} = [unsafe.Sizeof(jobAccounting{})]struct{}{}

// tree is what the runner needs to stop the processes of one run. On Windows that
// is a job object that holds the plugin and everything it starts.
type tree struct {
	cmd         *exec.Cmd
	interactive bool

	// The field job is zero until started has created it. Release closes it.
	job windows.Handle
}

// prepare makes the plugin start suspended, so that it can be put in its job
// object before it runs any code. A plugin that is not interactive also gets a
// process group of its own, which is what lets the runner send it Ctrl-Break. An
// interactive plugin stays in the console group of the CLI, so Ctrl-C reaches it.
func prepare(cmd *exec.Cmd, interactive bool) *tree {
	flags := uint32(windows.CREATE_SUSPENDED)
	if !interactive {
		flags |= windows.CREATE_NEW_PROCESS_GROUP
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}

	return &tree{cmd: cmd, interactive: interactive}
}

// started creates the job object, puts the suspended plugin in it, and lets the
// plugin run. Any process the plugin starts from then on is in the job too.
//
// It must be called after the plugin has started and before Wait is called. The
// handle comes from the Process of the command, which stays open until Wait, so
// it cannot refer to another process even if the plugin is killed meanwhile. If
// a step fails the job is still held, and release closes it.
func (t *tree) started() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("could not create a job object: %w", err)
	}

	t.job = job

	// Closing the last handle of the job ends the processes in it. That is what
	// stops the plugin when the CLI itself dies, and release lifts it again when
	// the plugin exits by itself.
	if err := setLimitFlags(job, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE); err != nil {
		return fmt.Errorf("could not set the limits of the job object: %w", err)
	}

	var stepErr error

	err = t.cmd.Process.WithHandle(func(handle uintptr) {
		process := windows.Handle(handle)

		if err := windows.AssignProcessToJobObject(job, process); err != nil {
			stepErr = fmt.Errorf("could not put the plugin in its job object: %w", err)

			return
		}

		if err := resume(process); err != nil {
			stepErr = fmt.Errorf("could not resume the plugin: %w", err)
		}
	})
	if err != nil {
		return fmt.Errorf("could not get the handle of the plugin: %w", err)
	}

	return stepErr
}

// terminate sends Ctrl-Break to the group of a plugin that is not interactive.
// It is best effort: it fails when the CLI has no console, and the kill after the
// grace period does not depend on it. An interactive plugin is not asked, since
// Ctrl-C has already reached it through the console.
func (t *tree) terminate() {
	if t.interactive {
		return
	}

	_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(t.cmd.Process.Pid))
}

// kill ends every process in the job at once, with exit code 1. If the job cannot
// be ended it ends the plugin alone.
func (t *tree) kill() {
	if err := windows.TerminateJobObject(t.job, 1); err != nil {
		// Without the job at least the plugin itself must go, or the run would wait
		// for it for ever.
		_ = t.cmd.Process.Kill()
	}
}

// lingering reports whether the job still has a process in it, after the plugin
// itself has gone.
func (t *tree) lingering() bool {
	var info jobAccounting

	err := queryJobInformation(t.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))

	return jobLingering(info.ActiveProcesses, err)
}

// release closes the handle of the job. When killRest is false the plugin exited
// by itself, and what it left running keeps running, as it does on Unix, so the
// kill on close is lifted first. If that fails the close ends them: the safe way
// to fail.
func (t *tree) release(killRest bool) {
	if t.job == 0 {
		return
	}

	if !killRest {
		_ = setLimitFlags(t.job, 0)
	}

	_ = windows.CloseHandle(t.job)
	t.job = 0
}

// exitCode returns the exit code of a finished process.
func exitCode(state *os.ProcessState) int {
	return mapExitCode(uint32(state.ExitCode()))
}

// resume lets every thread of a suspended process run.
func resume(process windows.Handle) error {
	if err := ntResumeProcess.Find(); err != nil {
		return err
	}

	status, _, _ := ntResumeProcess.Call(uintptr(process))

	// An NTSTATUS is a failure when it is negative.
	if int32(status) < 0 {
		return fmt.Errorf("NtResumeProcess failed with status %#x", uint32(status))
	}

	return nil
}

// setLimitFlags sets the limit flags of the job and nothing else.
func setLimitFlags(job windows.Handle, flags uint32) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION

	info.BasicLimitInformation.LimitFlags = flags

	return setJobInformation(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
}

// setJobInformation calls SetInformationJobObject. The directive keeps what info
// points to alive and in place for the duration of the call, as it does for the
// arguments of syscall.Syscall.
//
//go:uintptrescapes
func setJobInformation(job windows.Handle, class uint32, info uintptr, size uint32) error {
	_, err := windows.SetInformationJobObject(job, class, info, size)

	return err
}

// queryJobInformation calls QueryInformationJobObject, with the same directive.
//
//go:uintptrescapes
func queryJobInformation(job windows.Handle, class int32, info uintptr, size uint32) error {
	return windows.QueryInformationJobObject(job, class, info, size, nil)
}

package runner

import "syscall"

// controlCExit is the exit code Windows gives a process that Ctrl-C or Ctrl-Break
// ended (STATUS_CONTROL_C_EXIT).
const controlCExit = 0xC000013A

// mapExitCode turns the exit code of a Windows process into the code of an
// ExitError. A process that Ctrl-C ended gets 128 plus the number of SIGINT, as
// it does on Unix, and every other code is kept.
func mapExitCode(code uint32) int {
	if code == controlCExit {
		return 128 + int(syscall.SIGINT)
	}

	return int(code)
}

// jobLingering reports whether a job object still has a process in it, from the
// number of active processes and the error of the query. A query that failed
// counts as lingering, so the runner waits for the kill instead of trusting a
// number it does not have.
func jobLingering(active uint32, err error) bool {
	return err != nil || active > 0
}

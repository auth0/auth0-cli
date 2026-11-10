package cli

import (
	"golang.org/x/sys/windows"
)

// stillActive is the exit code that GetExitCodeProcess reports for a process
// that has not ended.
const stillActive = 259

// processAlive reports whether a process with the ID is still running. A process
// that has ended can stay in the table while something holds a handle to it, so
// the exit code decides, not whether the process can be opened.
func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}

	defer func() { _ = windows.CloseHandle(handle) }()

	var code uint32

	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}

	return code == stillActive
}

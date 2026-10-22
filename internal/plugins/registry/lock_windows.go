package registry

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes an exclusive lock on the file without waiting. It returns false
// and no error when another holder has the lock.
func tryLock(file *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return false, nil
	default:
		return false, err
	}
}

// unlock frees the lock that tryLock took.
func unlock(file *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, new(windows.Overlapped))
}

//go:build !windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock takes an exclusive lock on the file without waiting. It returns false
// and no error when another holder has the lock.
func tryLock(file *os.File) (bool, error) {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)

		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, unix.EINTR):
		case errors.Is(err, unix.EWOULDBLOCK):
			return false, nil
		default:
			return false, err
		}
	}
}

// unlock frees the lock that tryLock took.
func unlock(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

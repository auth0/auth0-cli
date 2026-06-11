//go:build !windows

package skills

import (
	"os"

	"golang.org/x/sys/unix"
)

func acquireSharedFlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_SH)
}

func acquireExclusiveFlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX)
}

func releaseFlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

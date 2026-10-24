package plugins

import (
	"errors"
	"syscall"
	"time"
)

// The Win32 error codes that a rename reports while another process holds the
// file or directory open without sharing it for deletion. They are written as
// numbers because the windows package only builds on Windows, and the retry
// logic is tested everywhere. On Windows they are the same values as
// windows.ERROR_ACCESS_DENIED, ERROR_SHARING_VIOLATION and ERROR_LOCK_VIOLATION.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

// The retry policy of Rename: at most renameTries attempts, with a wait after
// each failure that starts at renameFirstDelay, doubles, and stops growing at
// renameMaxDelay. The waits add up to about 1.3 seconds.
const (
	renameTries      = 10
	renameFirstDelay = 20 * time.Millisecond
	renameMaxDelay   = 200 * time.Millisecond
)

// isRetryable reports whether err is one that a later attempt at the same rename
// can get past, because a reader that has the file open closes it soon.
func isRetryable(err error) bool {
	return errors.Is(err, errorAccessDenied) ||
		errors.Is(err, errorSharingViolation) ||
		errors.Is(err, errorLockViolation)
}

// renameDelay returns how long to wait after the failed attempt with the given
// number, which starts at zero.
func renameDelay(attempt int) time.Duration {
	// Four doublings already pass the cap, and limiting the shift keeps a large
	// attempt number from overflowing.
	return min(renameFirstDelay<<min(attempt, 4), renameMaxDelay)
}

// retryRename calls rename until it succeeds, fails with an error that is not
// retryable, or has been tried renameTries times. It waits between attempts with
// sleep and returns the error of the last attempt.
func retryRename(rename func(oldpath, newpath string) error, sleep func(time.Duration), oldpath, newpath string) error {
	for attempt := 0; ; attempt++ {
		err := rename(oldpath, newpath)
		if err == nil || !isRetryable(err) || attempt == renameTries-1 {
			return err
		}

		sleep(renameDelay(attempt))
	}
}

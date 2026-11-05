// Package filelock provides the advisory lock that auth0 processes use to take
// turns when they change the plugin files on disk: flock on Unix and
// LockFileEx on Windows.
package filelock

import (
	"context"
	"fmt"
	"os"
	"time"
)

const (
	// The permissions of a lock file, which only the current user needs.
	filePerm os.FileMode = 0o600

	// How long to wait between two attempts to take the lock.
	pollInterval = 50 * time.Millisecond

	// How long to wait for the lock at most. A holder that is still busy after
	// this time makes Acquire give up with the busy error instead of waiting for
	// it.
	timeout = 30 * time.Second
)

// Acquire takes an exclusive lock on the file at path, which is created with
// restricted permissions when it does not exist, and returns the function that
// frees it. The file is created once and never removed, because removing it
// would let two processes lock different files with the same name. The
// operating system also frees the lock when the process ends, so a crash cannot
// leave it behind. Locks on separate handles of the same file exclude each
// other, also within one process.
//
// It tries without blocking and waits 50 milliseconds between attempts, until
// the context is done or 30 seconds have passed. It then returns busy, which
// the caller supplies as its own error for a lock that stays held, and also
// wraps the error of the context when that is what ended the wait. Any other
// error means the lock could not be tried at all, and does not wrap busy.
func Acquire(ctx context.Context, path string, busy error) (release func(), err error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open the lock file: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		locked, err := tryLock(file)
		if err != nil {
			_ = file.Close()

			return nil, fmt.Errorf("failed to lock %s: %w", path, err)
		}

		if locked {
			// Freeing the lock does what it can and reports nothing, as closing the
			// file frees it too.
			return func() {
				unlock(file)
				_ = file.Close()
			}, nil
		}

		select {
		case <-waitCtx.Done():
			_ = file.Close()

			if ctx.Err() != nil {
				return nil, fmt.Errorf("%w (%w)", busy, ctx.Err())
			}

			return nil, busy
		case <-ticker.C:
		}
	}
}

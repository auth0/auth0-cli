package registry

import (
	"context"
	"fmt"
	"os"
	"time"
)

const (
	// The name of the file in the cache directory that processes lock while they
	// refresh the registry. It is created once and never removed, because removing
	// it would let two processes lock different files with the same name.
	lockFileName = ".lock"

	// How long to wait between two attempts to take the lock.
	lockPollInterval = 50 * time.Millisecond

	// How long to wait for the lock at most. A refresh that is already running can
	// outlast it, because a refresh makes up to two attempts of two requests, each
	// of which may take as long as the request timeout of the HTTP client, so a
	// process that waits this long gives up with ErrBusy instead of waiting for
	// the whole refresh.
	lockTimeout = 30 * time.Second
)

// acquireLock takes an exclusive lock on the file at path, which is created with
// restricted permissions when it does not exist, and returns the function that
// frees it. The operating system also frees the lock when the process ends, so a
// crash cannot leave it behind. Locks on separate handles of the same file
// exclude each other, also within one process.
//
// It tries without blocking and waits lockPollInterval between attempts, until
// the context is done or lockTimeout has passed. It then returns an error that
// wraps ErrBusy, and also wraps the error of the context when that is what ended
// the wait.
func acquireLock(ctx context.Context, path string) (release func(), err error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, cacheFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open the registry lock: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	ticker := time.NewTicker(lockPollInterval)
	defer ticker.Stop()

	for {
		locked, err := tryLock(file)
		if err != nil {
			_ = file.Close()

			return nil, fmt.Errorf("failed to lock the registry cache: %w", err)
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
				return nil, fmt.Errorf("%w (%w)", ErrBusy, ctx.Err())
			}

			return nil, ErrBusy
		case <-ticker.C:
		}
	}
}

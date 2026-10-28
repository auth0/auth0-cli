package registry

import "time"

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

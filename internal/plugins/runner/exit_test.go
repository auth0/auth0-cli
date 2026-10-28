package runner

import "syscall"

// exitHelper ends a helper process without the exit hook of os.Exit. In a test
// binary built with coverage that hook warns on stderr that GOCOVERDIR is not set,
// because a helper runs with a minimal environment, and the warning would end up
// in the output that the tests compare.
func exitHelper(code int) {
	syscall.Exit(code)
}

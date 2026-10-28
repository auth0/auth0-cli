package cli

import "syscall"

// exitFixture ends the process of the fixture plugin without the exit hook of
// os.Exit. In a test binary built with coverage that hook warns on stderr that
// GOCOVERDIR is not set, because the fixture runs with a minimal environment, and
// the warning would end up in the output that the tests compare.
func exitFixture(code int) {
	syscall.Exit(code)
}

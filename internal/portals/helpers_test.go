package portals_test

import "fmt"

// wrapped adds context to an error, as the code around a store does.
func wrapped(err error) error {
	return fmt.Errorf("check the permissions: %w", err)
}

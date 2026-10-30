package registry

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// registryError is an error that is a constant, so that the sentinel errors
// below cannot be changed. Their messages are sentences that are shown to users
// as they are.
type registryError string

// Error returns the message of the error.
func (e registryError) Error() string {
	return string(e)
}

// The errors that fetching, verifying and loading a registry index can return.
// Use errors.Is to tell them apart, as most of them wrap more detail.
const (
	// ErrSignatureInvalid means the index is not signed by a trusted key. The
	// index is never used when this happens.
	ErrSignatureInvalid = registryError("The auth0 registry index failed signature verification.")

	// ErrNoTrustedKeys means the build embeds no trusted registry key, so no
	// index can be verified.
	ErrNoTrustedKeys = registryError("no trusted registry keys are embedded in this build")

	// ErrExpired means the index is past its expiry time.
	ErrExpired = registryError("The registry index expired")

	// ErrRollback means the index is older than one that was already accepted,
	// which can be a sign of a replay attack.
	ErrRollback = registryError("The registry index is older than one that was already accepted.")

	// ErrUnsupportedFormat means the index uses a format that is newer than this
	// CLI understands.
	ErrUnsupportedFormat = registryError("The registry index uses a newer format. Update the auth0 CLI to use it.")

	// ErrInvalidIndex means the index, or a per-plugin file that it points to,
	// does not follow the index format. The error also wraps a
	// *plugins.ValidationError that lists every problem found.
	ErrInvalidIndex = registryError("The registry index is not valid")

	// ErrOffline means the registry could not be reached or answered with an
	// error. The cached index, if there is a usable one, can still be used.
	ErrOffline = registryError("The registry could not be reached")

	// ErrHashMismatch means a per-plugin file does not have the hash that the
	// index pins for it.
	ErrHashMismatch = registryError("The plugin file does not match the hash pinned in the registry index")

	// ErrNotCached means there is no complete cached copy of the index yet.
	ErrNotCached = registryError("The registry index has not been downloaded yet.")

	// ErrBusy means another auth0 process held the lock of the registry cache for
	// too long, so this process did not refresh it.
	ErrBusy = registryError("Another auth0 process is refreshing the registry. Try again in a moment.")

	// ErrInsecureURL means a URL that the registry flow would use is not an
	// https URL.
	ErrInsecureURL = registryError("the URL must use https")
)

// newInvalidError returns an error that wraps ErrInvalidIndex and lists the
// problems found in what, which names the file the problems were found in.
func newInvalidError(what string, problems []string) error {
	return fmt.Errorf("%w (%s):\n%w", ErrInvalidIndex, what, &plugins.ValidationError{Errors: problems})
}

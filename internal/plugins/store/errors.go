package store

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// storeError is an error that is a constant, so that the sentinel errors below
// cannot be changed. Their messages are sentences that are shown to users as
// they are, and most of them are wrapped with the detail of what went wrong.
type storeError string

// Error returns the message of the error.
func (e storeError) Error() string {
	return string(e)
}

// The errors that installing, verifying and removing plugins can return. Use
// errors.Is to tell them apart, as most of them wrap more detail.
const (
	// ErrBusy means another auth0 process held the store lock for too long.
	ErrBusy = storeError("Another auth0 process is changing the installed plugins. Try again in a moment.")

	// ErrChanged means another auth0 process changed the plugin after the consent
	// for the change was decided, so the answer no longer applies. Nothing was
	// changed, and the command can be run again.
	ErrChanged = storeError("Another auth0 process changed this plugin while the change was being approved. Run the command again.")

	// ErrDownloadHost means a download URL, or a redirect on the way, is not an
	// https URL on one of the hosts that plugins are downloaded from.
	ErrDownloadHost = storeError("Plugins can only be downloaded over https from GitHub release hosts.")

	// ErrDownloadFailed means the file could not be downloaded, or the server
	// did not hand it out.
	ErrDownloadFailed = storeError("The plugin could not be downloaded. The registry lists it, but the file is unavailable.")

	// ErrDownloadTooLarge means the file is bigger than the registry says, or
	// bigger than the registry allows downloads to be.
	ErrDownloadTooLarge = storeError("The download is larger than allowed. The file was discarded.")

	// ErrDownloadSizeMismatch means the file is smaller than the registry says.
	ErrDownloadSizeMismatch = storeError("Downloaded file does not have the size the registry lists. The file was discarded.")

	// ErrDownloadHashMismatch means the file does not have the hash that the
	// registry lists for it.
	ErrDownloadHashMismatch = storeError("Downloaded file does not match the registry hash. The file was discarded.")

	// ErrInvalidArchive means the downloaded archive cannot be read, or holds
	// more than the plugin and its manifest, or holds them in a way that is not
	// allowed. Nothing from it is installed.
	ErrInvalidArchive = storeError("The plugin archive is not a valid plugin archive. Nothing was installed.")

	// ErrArtifactMismatch means the manifest inside the archive does not agree
	// with the registry record of the version.
	ErrArtifactMismatch = storeError("The artifact does not match the registry record. Nothing was installed.")

	// ErrLockfileDamaged means the lockfile cannot be read. It is never replaced
	// automatically.
	ErrLockfileDamaged = storeError("The plugin lockfile is damaged.")
)

// shortName returns the name of a plugin ID such as auth0/checkmate.
func shortName(id string) string {
	_, name, _ := strings.Cut(id, "/")

	return name
}

// registryName returns the registry of a plugin ID such as auth0/checkmate.
func registryName(id string) string {
	registry, _, _ := strings.Cut(id, "/")

	return registry
}

// NotAvailableError means the registry does not list the plugin, or does not
// list the version that was asked for.
type NotAvailableError struct {
	ID string

	// Version is the version that was asked for, or empty when the plugin itself
	// is not listed.
	Version string

	// Available lists the versions that can be installed, latest first.
	Available []string
}

// Error returns the message of the error.
func (e *NotAvailableError) Error() string {
	if e.Version == "" {
		return fmt.Sprintf("%s is not listed in the %s registry.", shortName(e.ID), registryName(e.ID))
	}

	message := fmt.Sprintf("%s %s is not available in the %s registry.", shortName(e.ID), e.Version, registryName(e.ID))
	if len(e.Available) == 0 {
		return message
	}

	listed := append([]string(nil), e.Available...)
	listed[0] += " (latest)"

	return message + "\nAvailable versions: " + strings.Join(listed, ", ")
}

// NoBuildError means the version has no build for this machine.
type NoBuildError struct {
	ID      string
	Version string

	// Machine names this machine, for example darwin-x64.
	Machine string

	// Platforms are the platforms that the version has builds for.
	Platforms []plugins.Platform

	// Versions lists the versions of the plugin that have a build for this
	// machine, latest first.
	Versions []string
}

// Error returns the message of the error.
func (e *NoBuildError) Error() string {
	platforms := make([]string, 0, len(e.Platforms))
	for _, platform := range e.Platforms {
		platforms = append(platforms, string(platform))
	}

	message := fmt.Sprintf("%s %s has no build for %s. It supports %s.", shortName(e.ID), e.Version, e.Machine, strings.Join(platforms, ", "))
	if len(e.Versions) > 0 {
		message += fmt.Sprintf(" Versions with a %s build: %s.", e.Machine, strings.Join(e.Versions, ", "))
	}

	return message
}

// EnginesError means the version needs another version of the auth0 CLI.
type EnginesError struct {
	ID         string
	Version    string
	Requires   string
	CLIVersion string
}

// Error returns the message of the error.
func (e *EnginesError) Error() string {
	return fmt.Sprintf("%s %s requires auth0-cli %s. You have %s.", shortName(e.ID), e.Version, e.Requires, e.CLIVersion)
}

// UnsupportedAPIVersionError means the plugin targets a plugin API that this
// CLI does not support. The registry does not list the API version, so it is
// found once the manifest has been extracted.
type UnsupportedAPIVersionError struct {
	ID         string
	Version    string
	APIVersion int
}

// Error returns the message of the error.
func (e *UnsupportedAPIVersionError) Error() string {
	return fmt.Sprintf("%s %s targets plugin API version %d, which needs a newer auth0 CLI. Nothing was installed.", shortName(e.ID), e.Version, e.APIVersion)
}

// RevokedError means the version was revoked and must not be installed.
type RevokedError struct {
	ID      string
	Version string
	Reason  string
}

// Error returns the message of the error.
func (e *RevokedError) Error() string {
	return fmt.Sprintf("%s %s was revoked: %s. Install a newer version with: auth0 plugin update %s", shortName(e.ID), e.Version, strings.TrimRight(e.Reason, ". "), shortName(e.ID))
}

// UnsupportedFormatError means the artifact is packaged in a format that this
// CLI does not install.
type UnsupportedFormatError struct {
	ID      string
	Version string
	Format  string
}

// Error returns the message of the error.
func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf("%s %s uses an unsupported artifact format (%s). Only tar.gz and zip artifacts can be installed.", shortName(e.ID), e.Version, e.Format)
}

// NotInstalledError means the plugin is not installed.
type NotInstalledError struct {
	ID string
}

// Error returns the message of the error.
func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("%s is not installed. Install it with: auth0 plugin install %s", shortName(e.ID), shortName(e.ID))
}

// DeclinedError means the Consent callback refused a change, so nothing was
// downloaded and the installed version stays active.
type DeclinedError struct {
	ID      string
	Version string

	// Err is what the callback returned.
	Err error
}

// Error returns the message of the error.
func (e *DeclinedError) Error() string {
	return fmt.Sprintf("%s %s was not approved, so nothing was changed: %v", shortName(e.ID), e.Version, e.Err)
}

// Unwrap returns what the callback returned.
func (e *DeclinedError) Unwrap() error {
	return e.Err
}

// VerificationError means the files of an installed plugin are not the ones that
// were verified at install time, so the plugin must not run.
type VerificationError struct {
	ID string

	// Reason says what does not match.
	Reason string
}

// Error returns the message of the error.
func (e *VerificationError) Error() string {
	return fmt.Sprintf("%s no longer matches what was verified when it was installed (%s), so it will not run. Reinstall it with: auth0 plugin install %s", shortName(e.ID), e.Reason, shortName(e.ID))
}

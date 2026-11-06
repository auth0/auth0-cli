package portals

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// APIError is a failed Management API request, whatever client made it.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Code is the errorCode the API returned, if any.
	Code string
	// Message is the message the API returned, if any.
	Message string
}

func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
	case e.Message != "":
		return fmt.Sprintf("%d: %s", e.Status, e.Message)
	default:
		return fmt.Sprintf("%d: %s", e.Status, http.StatusText(e.Status))
	}
}

// StatusOf returns the HTTP status of an API failure in err's chain, or 0.
func StatusOf(err error) int {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Status
	}

	return 0
}

// CodeOf returns the errorCode of an API failure in err's chain, or "".
func CodeOf(err error) string {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Code
	}

	return ""
}

// IsStatus reports whether err is an API failure with one of the given statuses.
func IsStatus(err error, statuses ...int) bool {
	status := StatusOf(err)
	if status == 0 {
		return false
	}

	for _, want := range statuses {
		if status == want {
			return true
		}
	}

	return false
}

// FeatureNotEnabledError means Universal Portals is not enabled for the tenant.
type FeatureNotEnabledError struct {
	Tenant string
}

func (e *FeatureNotEnabledError) Error() string {
	return fmt.Sprintf(
		"Universal Portals is not enabled for %s. Ask Auth0 to enable it for this tenant, then run the command again",
		e.Tenant,
	)
}

// MissingScopesError means the session lacks permissions setup needs. Setup checks them before it
// creates anything.
type MissingScopesError struct {
	// Scopes are the scopes that are all needed.
	Scopes []string
	// Choices are the checks where any one of several scopes would do.
	Choices []ScopeGap
}

func (e *MissingScopesError) needs() []string {
	needs := slices.Clone(e.Scopes)
	for _, choice := range e.Choices {
		needs = append(needs, "one of "+strings.Join(choice.Scopes, " or "))
	}

	return needs
}

func (e *MissingScopesError) Error() string {
	needs := e.needs()

	return fmt.Sprintf(
		"The CLI session is missing %d permission(s) setup needs: %s. Nothing was created",
		len(needs),
		strings.Join(needs, ", "),
	)
}

// Fixes returns how to get the missing scopes. When any one of several scopes is enough, the
// first is the one to ask for.
func (e *MissingScopesError) Fixes() []string {
	if len(e.needs()) == 0 {
		return nil
	}

	missing := slices.Clone(e.Scopes)
	for _, choice := range e.Choices {
		missing = append(missing, choice.Required()...)
	}

	return ScopeFixes(missing)
}

// PermissionCheckError means the Management API refused a check with a 403 and no reason. It
// usually means Universal Portals is not enabled, and setup stops before it creates anything.
type PermissionCheckError struct {
	Probes []string
	Tenant string
}

func (e *PermissionCheckError) Error() string {
	quoted := make([]string, 0, len(e.Probes))
	for _, probe := range e.Probes {
		quoted = append(quoted, fmt.Sprintf("%q", probe))
	}

	return fmt.Sprintf(
		"The Management API refused %s with HTTP 403 and gave no reason. Nothing was created. "+
			"Usually Universal Portals is not enabled for %s, or the session is missing a permission",
		strings.Join(quoted, ", "),
		e.Tenant,
	)
}

// Fixes returns what to check.
func (e *PermissionCheckError) Fixes() []string {
	return append([]string{"Check that Universal Portals is enabled for this tenant."}, ScopeFixes(nil)...)
}

// fixable is an error that knows how to be fixed.
type fixable interface {
	error
	Fixes() []string
}

// FixesOf returns the commands or steps that resolve an error: the fixes the error names, or, for a
// request the API refused for a missing scope, how to get the scope. Whichever command hit it, a
// missing scope is fixed the same way.
func FixesOf(err error) []string {
	if fixer, ok := errors.AsType[fixable](err); ok {
		return fixer.Fixes()
	}

	if gap, isGap := ParseScopeGap("", err); isGap {
		return ScopeFixes(gap.Required())
	}

	return nil
}

// UsageError is a command that cannot run as asked, for example a slug the API would refuse.
type UsageError struct {
	Message string
	// Hints are the steps that resolve it.
	Hints []string
}

func (e *UsageError) Error() string { return e.Message }

// Fixes returns the steps that resolve the error.
func (e *UsageError) Fixes() []string { return e.Hints }

// NameInUseError means an application with the name exists and was not created by this setup, so it
// is never reused.
type NameInUseError struct {
	Name string
}

func (e *NameInUseError) Error() string {
	return fmt.Sprintf(
		"An application named %q already exists and was not created by this setup. Rename or delete it, then rerun",
		e.Name,
	)
}

// SlugTakenError means another portal uses the slug.
type SlugTakenError struct {
	Slug string
}

func (e *SlugTakenError) Error() string {
	return fmt.Sprintf("The portal slug %q is already used by another portal", e.Slug)
}

// SlugRaceError means another portal took the slug while setup was running. Setup checks the slug
// before it creates anything, so this is a race, and the rerun needs a new name and slug.
type SlugRaceError struct {
	Slug string
}

func (e *SlugRaceError) Error() string {
	return fmt.Sprintf("The slug %q was taken while setup was running. Rerun with a different --name and --slug", e.Slug)
}

// ClientSecretUnreadableError means the secret of an application could not be read, which takes the
// read:client_keys scope.
type ClientSecretUnreadableError struct {
	ClientID string
}

func (e *ClientSecretUnreadableError) Error() string {
	return fmt.Sprintf("Could not read the client secret of application %q. The session needs read:client_keys", e.ClientID)
}

// Fixes returns how to get the scope.
func (e *ClientSecretUnreadableError) Fixes() []string {
	return ScopeFixes([]string{"read:client_keys"})
}

// asSlugTaken reports whether err is a *SlugTakenError.
func asSlugTaken(err error) (*SlugTakenError, bool) {
	return errors.AsType[*SlugTakenError](err)
}

// BusinessAccessError means the user who will test a business portal cannot be used, or has no
// access to an organization. Setup checks it before it creates anything.
type BusinessAccessError struct {
	Message string
	// Hints are the steps that resolve it.
	Hints []string
}

func (e *BusinessAccessError) Error() string { return e.Message }

// Fixes returns the steps that resolve the error.
func (e *BusinessAccessError) Fixes() []string { return e.Hints }

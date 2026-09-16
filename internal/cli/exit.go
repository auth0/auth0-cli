package cli

import (
	"encoding/json"
	"errors"

	"github.com/auth0/go-auth0/management"
	"github.com/auth0/go-auth0/v3/management/core"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

// Process exit codes. These form part of the CLI's public contract: agents and
// scripts branch on the failure class without parsing output. See
// AGENT_COMPATIBILITY_PLAN.md §7.
const (
	exitOK          = 0
	exitGeneric     = 1
	exitUsage       = 2
	exitAuth        = 3
	exitValidation  = 4
	exitNotFound    = 5
	exitRateLimit   = 6
	exitAPI         = 7
	exitInterrupted = 130
)

// usageError wraps a command-usage failure (bad flag, unknown flag) so it maps
// to the dedicated usage exit code. Flag parse errors are wrapped via cobra's
// FlagErrorFunc; see buildRootCmd.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }

func (e usageError) Unwrap() error { return e.err }

// authError wraps an authentication/authorization setup failure (expired token
// in --no-input mode, corrupted token, failed credential refresh) so it maps to
// the auth exit code, letting agents detect "must re-authenticate" from the code
// alone instead of scraping the message.
type authError struct{ err error }

func (e authError) Error() string { return e.err.Error() }

func (e authError) Unwrap() error { return e.err }

// validationError wraps a client-side input failure (unreadable/malformed JSON,
// local schema validation) so it maps to the validation exit code before any API
// call is made, matching the class a server-side 400/422 would produce.
type validationError struct{ err error }

func (e validationError) Error() string { return e.err.Error() }

func (e validationError) Unwrap() error { return e.err }

// errorClass returns a stable, machine-readable classification for an error. It
// is the single source of truth shared by exit-code mapping, the JSON error
// envelope, and analytics tracking.
func errorClass(err error) string {
	if err == nil {
		return "none"
	}

	var usageErr usageError
	if errors.As(err, &usageErr) {
		return "usage"
	}

	var authErr authError
	if errors.As(err, &authErr) {
		return "auth"
	}

	var validationErr validationError
	if errors.As(err, &validationErr) {
		return "validation"
	}

	if errors.Is(err, config.ErrInvalidToken) ||
		errors.Is(err, config.ErrMalformedToken) ||
		errors.Is(err, config.ErrNoAuthenticatedTenants) ||
		errors.Is(err, config.ErrConfigFileMissing) {
		return "auth"
	}

	var missingScopesErr config.ErrTokenMissingRequiredScopes
	if errors.As(err, &missingScopesErr) {
		return "auth"
	}

	if status, ok := managementHTTPStatus(err); ok {
		return errorClassForHTTPStatus(status)
	}

	return "unknown"
}

// exitCodeForError maps an error onto its process exit code per the granular scheme.
func exitCodeForError(err error) int {
	if err == nil {
		return exitOK
	}

	switch errorClass(err) {
	case "usage":
		return exitUsage
	case "auth":
		return exitAuth
	case "validation":
		return exitValidation
	case "not_found":
		return exitNotFound
	case "rate_limit":
		return exitRateLimit
	case "api":
		return exitAPI
	default:
		return exitGeneric
	}
}

// errorDetailer lets an error contribute structured details (e.g. field-level
// validation errors) to the JSON error envelope's "details" field.
type errorDetailer interface {
	ErrorDetails() json.RawMessage
}

// errorDetails extracts structured details from an error chain, if any error in
// it implements errorDetailer.
func errorDetails(err error) json.RawMessage {
	var detailer errorDetailer
	if errors.As(err, &detailer) {
		return detailer.ErrorDetails()
	}

	return nil
}

// errorHTTPStatus returns the HTTP status carried by an error, if any.
func errorHTTPStatus(err error) int {
	if status, ok := managementHTTPStatus(err); ok {
		return status
	}

	return 0
}

// buildErrorEnvelope assembles the machine-readable error emitted on stderr in
// JSON/agent mode.
func buildErrorEnvelope(err error) display.ErrorEnvelope {
	return display.ErrorEnvelope{
		Error: display.ErrorBody{
			Code:    errorClass(err),
			Message: err.Error(),
			Status:  errorHTTPStatus(err),
			Details: errorDetails(err),
		},
	}
}

// managementHTTPStatus extracts the HTTP status from a go-auth0 management API
// error anywhere in the error chain, supporting both the v1 (management.Error)
// and v3 (*core.APIError) SDK error types.
func managementHTTPStatus(err error) (int, bool) {
	var v1 management.Error
	if errors.As(err, &v1) {
		return v1.Status(), true
	}

	var v3 *core.APIError
	if errors.As(err, &v3) {
		return v3.StatusCode, true
	}

	return 0, false
}

// errorClassForHTTPStatus maps an HTTP status onto a coarse failure class.
func errorClassForHTTPStatus(status int) string {
	switch {
	case status == 401 || status == 403:
		return "auth"
	case status == 400 || status == 422:
		return "validation"
	case status == 404:
		return "not_found"
	case status == 429:
		return "rate_limit"
	case status >= 500:
		return "api"
	default:
		return "unknown"
	}
}

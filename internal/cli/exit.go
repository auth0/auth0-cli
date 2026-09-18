package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/auth0/go-auth0/management"
	"github.com/auth0/go-auth0/v3/management/core"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

// The machine-readable failure class (usage, auth, not_found, ...) Is carried by
// the JSON error envelope in agent/JSON mode (see buildErrorEnvelope), not by the
// exit code, so scripts that only check "0 vs non-zero" keep working.
const (
	exitOK          = 0
	exitGeneric     = 1
	exitInterrupted = 130
)

// usageError wraps a command-usage failure (bad flag, unknown flag) so it
// classifies as "usage" in the JSON error envelope. Flag parse errors are wrapped
// via cobra's FlagErrorFunc; see buildRootCmd. The optional reason carries a
// finer sub-classification (see errorReason) that is surfaced in analytics and
// the envelope's "reason" field; an empty reason falls back to a class default.
type usageError struct {
	err    error
	reason string
}

func (e usageError) Error() string { return e.err.Error() }

func (e usageError) Unwrap() error { return e.err }

// unknownCommandError is the usage failure for a mistyped command or subcommand
// (for example `auth0 appps` or `auth0 apps shoe`). Its Error() stays a single
// line so the JSON error envelope keeps a clean, machine-parseable message, while
// the "did you mean" suggestions are carried separately so the human renderer can
// show a hint block without polluting the agent-facing message. It unwraps to a
// usageError so it classifies as "usage" for the envelope, exit code and analytics.
type unknownCommandError struct {
	token       string
	parent      string
	suggestions []string
}

func (e unknownCommandError) Error() string {
	return fmt.Sprintf("unknown command %q for %q", e.token, e.parent)
}

func (e unknownCommandError) Unwrap() error { return usageError{err: errors.New(e.Error())} }

// authError wraps an authentication/authorization setup failure (expired token
// in --no-input mode, corrupted token, failed credential refresh) so it
// classifies as "auth" in the JSON error envelope, letting agents detect "must
// re-authenticate" from the code alone instead of scraping the message. The
// optional reason carries a finer sub-classification (see errorReason).
type authError struct {
	err    error
	reason string
}

func (e authError) Error() string { return e.err.Error() }

func (e authError) Unwrap() error { return e.err }

// validationError wraps a client-side input failure (unreadable/malformed JSON,
// local schema validation, invalid flag values) so it classifies as "validation"
// in the JSON error envelope before any API call is made, matching the class a
// server-side 400/422 would produce. When details is set it carries the
// field-level failures into the JSON error envelope's "details" field via the
// errorDetailer interface. The optional reason carries a finer sub-classification
// (see errorReason).
type validationError struct {
	err     error
	details json.RawMessage
	reason  string
}

func (e validationError) Error() string { return e.err.Error() }

func (e validationError) Unwrap() error { return e.err }

func (e validationError) ErrorDetails() json.RawMessage { return e.details }

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

	if isNetworkError(err) {
		return "network"
	}

	return "unknown"
}

// errorReason returns a finer sub-classification that lives alongside the coarse,
// stable errorClass. Where errorClass answers "what kind of failure", errorReason
// answers "why", walking the same ladder in the same order so the two never
// disagree. It is additive metadata (analytics error_reason, envelope "reason")
// and, unlike errorClass, is free to grow new values over time.
func errorReason(err error) string {
	if err == nil {
		return "none"
	}

	var usageErr usageError
	if errors.As(err, &usageErr) {
		if usageErr.reason != "" {
			return usageErr.reason
		}
		return "flag_parse"
	}

	// A tagged authError with an explicit reason always wins over the sentinel
	// defaults below, since the call site knows the specific cause.
	var authErr authError
	if errors.As(err, &authErr) && authErr.reason != "" {
		return authErr.reason
	}

	// Auth sentinels also classify as "auth" in errorClass; keep the same order.
	switch {
	case errors.Is(err, config.ErrNoAuthenticatedTenants):
		return "not_logged_in"
	case errors.Is(err, config.ErrConfigFileMissing):
		return "no_config"
	case errors.Is(err, config.ErrInvalidToken):
		return "session_expired"
	case errors.Is(err, config.ErrMalformedToken):
		return "token_malformed"
	}

	var missingScopesErr config.ErrTokenMissingRequiredScopes
	if errors.As(err, &missingScopesErr) {
		return "missing_scopes"
	}

	// A tagged authError with no explicit reason still classifies as auth.
	if errors.As(err, &authErr) {
		return "auth_failed"
	}

	var validationErr validationError
	if errors.As(err, &validationErr) {
		if validationErr.reason != "" {
			return validationErr.reason
		}
		return "local_validation"
	}

	if status, ok := managementHTTPStatus(err); ok {
		return reasonForHTTPStatus(status)
	}

	if isNetworkError(err) {
		return "transport"
	}

	return "unclassified"
}

// isNetworkError reports whether an error is a transport-level failure (DNS,
// dial, TLS, read/write) or a request timeout, so it classifies as "network"
// rather than the "unknown" fallthrough. A context cancellation (Ctrl-C) is
// deliberately excluded: that is user interruption, handled by the signal path.
func isNetworkError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// exitCodeForError maps an error onto its process exit code. Every failure
// collapses to the generic code so the CLI stays backwards compatible with
// scripts that treat any non-zero exit as failure; the granular failure class is
// still available to agents via the JSON error envelope (see buildErrorEnvelope).
// Interruption (Ctrl-C) exits 130 and is handled separately in the signal path.
func exitCodeForError(err error) int {
	if err == nil {
		return exitOK
	}

	return exitGeneric
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
	body := display.ErrorBody{
		Code:    errorClass(err),
		Reason:  errorReason(err),
		Message: err.Error(),
		Status:  errorHTTPStatus(err),
		Details: errorDetails(err),
	}

	// For a mistyped command, carry the "did you mean" candidates as structured
	// details so an agent gets the same hint the human renderer prints, without
	// having to parse it out of the message. Only fill this in when no error in
	// the chain already contributed richer details via errorDetailer.
	if body.Details == nil {
		var unknownCmd unknownCommandError
		if errors.As(err, &unknownCmd) && len(unknownCmd.suggestions) > 0 {
			if raw, marshalErr := json.Marshal(map[string]interface{}{"suggestions": unknownCmd.suggestions}); marshalErr == nil {
				body.Details = raw
			}
		}
	}

	return display.ErrorEnvelope{Error: body}
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
	case status == 400 || status == 422 || status == 410 || status == 415:
		return "validation"
	case status == 404:
		return "not_found"
	case status == 409:
		return "conflict"
	case status == 429:
		return "rate_limit"
	case status >= 500:
		return "api"
	default:
		return "unknown"
	}
}

// reasonForHTTPStatus maps an HTTP status onto the finer errorReason value that
// accompanies the coarse class from errorClassForHTTPStatus.
func reasonForHTTPStatus(status int) string {
	switch status {
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 400, 422:
		return "invalid_request"
	case 404:
		return "not_found"
	case 409:
		return "conflict"
	case 410:
		return "gone"
	case 415:
		return "unsupported_media_type"
	case 429:
		return "rate_limited"
	default:
		// Any 5xx, and any other unexpected status carried by an API error, is a
		// server-side failure from the caller's point of view.
		return "server_error"
	}
}

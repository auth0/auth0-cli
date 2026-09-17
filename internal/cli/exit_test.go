package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

// fakeManagementError implements the v1 management.Error interface (Status() int
// plus error) so exit-code and error-class mapping can be exercised without a
// live API call.
type fakeManagementError struct {
	status  int
	message string
}

func (e fakeManagementError) Status() int   { return e.status }
func (e fakeManagementError) Error() string { return e.message }

func TestErrorClass(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil error", err: nil, expected: "none"},
		{name: "usage error", err: usageError{err: errors.New("unknown flag")}, expected: "usage"},
		{name: "wrapped usage error", err: fmt.Errorf("wrap: %w", usageError{err: errors.New("bad flag")}), expected: "usage"},
		{name: "invalid token", err: config.ErrInvalidToken, expected: "auth"},
		{name: "malformed token", err: config.ErrMalformedToken, expected: "auth"},
		{name: "not logged in", err: config.ErrNoAuthenticatedTenants, expected: "auth"},
		{name: "config file missing", err: config.ErrConfigFileMissing, expected: "auth"},
		{name: "wrapped auth error", err: authError{err: fmt.Errorf("wrap: %w", errors.New("token expired"))}, expected: "auth"},
		{name: "local validation error", err: validationError{err: errors.New("schema validation failed")}, expected: "validation"},
		{name: "wrapped validation error", err: fmt.Errorf("wrap: %w", validationError{err: errors.New("bad json")}), expected: "validation"},
		{name: "missing scopes", err: config.ErrTokenMissingRequiredScopes{MissingScopes: []string{"read:users"}}, expected: "auth"},
		{name: "401 unauthorized", err: fakeManagementError{status: 401}, expected: "auth"},
		{name: "403 forbidden", err: fakeManagementError{status: 403}, expected: "auth"},
		{name: "400 bad request", err: fakeManagementError{status: 400}, expected: "validation"},
		{name: "422 unprocessable", err: fakeManagementError{status: 422}, expected: "validation"},
		{name: "404 not found", err: fakeManagementError{status: 404}, expected: "not_found"},
		{name: "429 rate limited", err: fakeManagementError{status: 429}, expected: "rate_limit"},
		{name: "500 server error", err: fakeManagementError{status: 500}, expected: "api"},
		{name: "503 server error", err: fakeManagementError{status: 503}, expected: "api"},
		{name: "unmapped status", err: fakeManagementError{status: 418}, expected: "unknown"},
		{name: "deadline exceeded", err: context.DeadlineExceeded, expected: "network"},
		{name: "wrapped deadline exceeded", err: fmt.Errorf("waiting: %w", context.DeadlineExceeded), expected: "network"},
		{name: "url error", err: &url.Error{Op: "Get", URL: "https://example", Err: errors.New("dial tcp: connection refused")}, expected: "network"},
		{name: "net timeout error", err: fakeNetError{timeout: true}, expected: "network"},
		{name: "context canceled is not network", err: context.Canceled, expected: "unknown"},
		{name: "generic error", err: errors.New("boom"), expected: "unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, errorClass(test.err))
		})
	}
}

// fakeNetError implements net.Error so the network classification can be
// exercised without opening a real socket.
type fakeNetError struct {
	timeout bool
}

func (e fakeNetError) Error() string   { return "simulated network failure" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return false }

// TestErrorReason exercises the finer sub-classification that accompanies the
// coarse errorClass. Every taxonomy row is covered, including the tagged-wrapper
// reasons and the sentinel/HTTP/network defaults.
func TestErrorReason(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil error", err: nil, expected: "none"},
		{name: "usage default", err: usageError{err: errors.New("bad flag")}, expected: "flag_parse"},
		{name: "usage tagged required flag", err: usageError{err: errors.New("required flag(s) \"x\" not set"), reason: "required_flag"}, expected: "required_flag"},
		{name: "auth tagged session expired", err: authError{err: errors.New("expired"), reason: "session_expired"}, expected: "session_expired"},
		{name: "auth tagged login failed", err: authError{err: errors.New("login"), reason: "login_failed"}, expected: "login_failed"},
		{name: "auth tagged client init", err: authError{err: errors.New("init"), reason: "client_init_failed"}, expected: "client_init_failed"},
		{name: "auth untagged", err: authError{err: errors.New("generic auth")}, expected: "auth_failed"},
		{name: "not logged in sentinel", err: config.ErrNoAuthenticatedTenants, expected: "not_logged_in"},
		{name: "config missing sentinel", err: config.ErrConfigFileMissing, expected: "no_config"},
		{name: "invalid token sentinel", err: config.ErrInvalidToken, expected: "session_expired"},
		{name: "malformed token sentinel", err: config.ErrMalformedToken, expected: "token_malformed"},
		{name: "missing scopes", err: config.ErrTokenMissingRequiredScopes{MissingScopes: []string{"read:users"}}, expected: "missing_scopes"},
		{name: "validation default", err: validationError{err: errors.New("bad json")}, expected: "local_validation"},
		{name: "401 unauthorized", err: fakeManagementError{status: 401}, expected: "unauthorized"},
		{name: "403 forbidden", err: fakeManagementError{status: 403}, expected: "forbidden"},
		{name: "400 invalid request", err: fakeManagementError{status: 400}, expected: "invalid_request"},
		{name: "422 invalid request", err: fakeManagementError{status: 422}, expected: "invalid_request"},
		{name: "404 not found", err: fakeManagementError{status: 404}, expected: "not_found"},
		{name: "429 rate limited", err: fakeManagementError{status: 429}, expected: "rate_limited"},
		{name: "500 server error", err: fakeManagementError{status: 500}, expected: "server_error"},
		{name: "418 server error", err: fakeManagementError{status: 418}, expected: "server_error"},
		{name: "network transport", err: context.DeadlineExceeded, expected: "transport"},
		{name: "generic unclassified", err: errors.New("boom"), expected: "unclassified"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, errorReason(test.err))
		})
	}
}

func TestExitCodeForError(t *testing.T) {
	// Exit codes are intentionally coarse: success is 0 and every failure class
	// collapses to the generic code, so scripts that only distinguish success from
	// failure keep working. The granular class lives in the JSON error envelope.
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "nil error", err: nil, expected: exitOK},
		{name: "usage error", err: usageError{err: errors.New("bad flag")}, expected: exitGeneric},
		{name: "auth error", err: config.ErrInvalidToken, expected: exitGeneric},
		{name: "server validation error", err: fakeManagementError{status: 400}, expected: exitGeneric},
		{name: "local validation error", err: validationError{err: errors.New("bad payload")}, expected: exitGeneric},
		{name: "not found error", err: fakeManagementError{status: 404}, expected: exitGeneric},
		{name: "rate limit error", err: fakeManagementError{status: 429}, expected: exitGeneric},
		{name: "api error", err: fakeManagementError{status: 500}, expected: exitGeneric},
		{name: "generic error", err: errors.New("boom"), expected: exitGeneric},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, exitCodeForError(test.err))
		})
	}
}

// TestAgentModeHelpMatchesExitCodes guards the help text against re-introducing
// a granular exit-code contract that exitCodeForError does not honor. Every
// failure collapses to the generic code, so the class lives in the JSON
// envelope's "code" field, not in the exit code.
func TestAgentModeHelpMatchesExitCodes(t *testing.T) {
	for _, class := range []string{"2 usage", "3 auth", "4 validation", "5 not-found", "6 rate-limit", "7 api"} {
		assert.NotContains(t, agentModeHelp, class, "help must not promise a distinct numeric exit code per failure class")
	}

	assert.Contains(t, agentModeHelp, "exits 1", "help should state the coarse failure exit code")
	assert.Contains(t, agentModeHelp, `"code" field`, "help should point the failure class at the JSON envelope")
}

func TestBuildErrorEnvelope(t *testing.T) {
	t.Run("classifies and carries the HTTP status", func(t *testing.T) {
		envelope := buildErrorEnvelope(fakeManagementError{status: 404, message: "404 Not Found: connection not found"})

		assert.Equal(t, "not_found", envelope.Error.Code)
		assert.Equal(t, "404 Not Found: connection not found", envelope.Error.Message)
		assert.Equal(t, 404, envelope.Error.Status)
	})

	t.Run("omits status for non-API errors", func(t *testing.T) {
		envelope := buildErrorEnvelope(usageError{err: errors.New("unknown flag --foo")})

		assert.Equal(t, "usage", envelope.Error.Code)
		assert.Equal(t, "unknown flag --foo", envelope.Error.Message)
		assert.Zero(t, envelope.Error.Status)
	})

	t.Run("carries the finer reason alongside the coarse code", func(t *testing.T) {
		envelope := buildErrorEnvelope(authError{err: errors.New("expired"), reason: "session_expired"})

		assert.Equal(t, "auth", envelope.Error.Code)
		assert.Equal(t, "session_expired", envelope.Error.Reason)
	})

	t.Run("omits the reason when it is empty", func(t *testing.T) {
		// A nil error yields no reason, so "reason" must be absent from the JSON.
		envelope := display.ErrorEnvelope{Error: display.ErrorBody{Code: "usage", Message: "x"}}

		raw, err := json.Marshal(envelope)
		assert.NoError(t, err)
		assert.NotContains(t, string(raw), "reason")
	})

	t.Run("marshals the reason field when set", func(t *testing.T) {
		envelope := buildErrorEnvelope(fakeManagementError{status: 429, message: "429 Too Many Requests"})

		raw, err := json.Marshal(envelope)
		assert.NoError(t, err)

		var decoded map[string]map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Equal(t, "rate_limited", decoded["error"]["reason"])
	})

	t.Run("marshals to the documented envelope shape", func(t *testing.T) {
		envelope := buildErrorEnvelope(fakeManagementError{status: 429, message: "429 Too Many Requests"})

		raw, err := json.Marshal(envelope)
		assert.NoError(t, err)

		var decoded map[string]map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Equal(t, "rate_limit", decoded["error"]["code"])
		assert.Equal(t, "429 Too Many Requests", decoded["error"]["message"])
		assert.Equal(t, float64(429), decoded["error"]["status"])
	})
}

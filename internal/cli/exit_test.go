package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
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
		{name: "usage error", err: usageError{errors.New("unknown flag")}, expected: "usage"},
		{name: "wrapped usage error", err: fmt.Errorf("wrap: %w", usageError{errors.New("bad flag")}), expected: "usage"},
		{name: "invalid token", err: config.ErrInvalidToken, expected: "auth"},
		{name: "malformed token", err: config.ErrMalformedToken, expected: "auth"},
		{name: "not logged in", err: config.ErrNoAuthenticatedTenants, expected: "auth"},
		{name: "config file missing", err: config.ErrConfigFileMissing, expected: "auth"},
		{name: "wrapped auth error", err: authError{fmt.Errorf("wrap: %w", errors.New("token expired"))}, expected: "auth"},
		{name: "local validation error", err: validationError{errors.New("schema validation failed")}, expected: "validation"},
		{name: "wrapped validation error", err: fmt.Errorf("wrap: %w", validationError{errors.New("bad json")}), expected: "validation"},
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
		{name: "generic error", err: errors.New("boom"), expected: "unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, errorClass(test.err))
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
		{name: "usage error", err: usageError{errors.New("bad flag")}, expected: exitGeneric},
		{name: "auth error", err: config.ErrInvalidToken, expected: exitGeneric},
		{name: "server validation error", err: fakeManagementError{status: 400}, expected: exitGeneric},
		{name: "local validation error", err: validationError{errors.New("bad payload")}, expected: exitGeneric},
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
		envelope := buildErrorEnvelope(usageError{errors.New("unknown flag --foo")})

		assert.Equal(t, "usage", envelope.Error.Code)
		assert.Equal(t, "unknown flag --foo", envelope.Error.Message)
		assert.Zero(t, envelope.Error.Status)
	})

	t.Run("carries did-you-mean suggestions as structured details", func(t *testing.T) {
		envelope := buildErrorEnvelope(unknownCommandError{
			token:       "appps",
			parent:      "auth0",
			suggestions: []string{"apps", "apis"},
		})

		assert.Equal(t, "usage", envelope.Error.Code)
		assert.Equal(t, `unknown command "appps" for "auth0"`, envelope.Error.Message)
		assert.Equal(t, map[string]interface{}{"suggestions": []string{"apps", "apis"}}, envelope.Error.Details)
	})

	t.Run("omits details when the unknown command has no suggestions", func(t *testing.T) {
		envelope := buildErrorEnvelope(unknownCommandError{token: "zzz", parent: "auth0"})

		assert.Equal(t, "usage", envelope.Error.Code)
		assert.Nil(t, envelope.Error.Details)
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

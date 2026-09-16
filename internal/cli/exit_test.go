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
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "nil error", err: nil, expected: exitOK},
		{name: "usage error", err: usageError{errors.New("bad flag")}, expected: exitUsage},
		{name: "auth error", err: config.ErrInvalidToken, expected: exitAuth},
		{name: "server validation error", err: fakeManagementError{status: 400}, expected: exitValidation},
		{name: "local validation error", err: validationError{errors.New("bad payload")}, expected: exitValidation},
		{name: "not found error", err: fakeManagementError{status: 404}, expected: exitNotFound},
		{name: "rate limit error", err: fakeManagementError{status: 429}, expected: exitRateLimit},
		{name: "api error", err: fakeManagementError{status: 500}, expected: exitAPI},
		{name: "generic error", err: errors.New("boom"), expected: exitGeneric},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, exitCodeForError(test.err))
		})
	}
}

func TestBuildErrorEnvelope(t *testing.T) {
	t.Run("classifies and carries the HTTP status", func(t *testing.T) {
		envelope := buildErrorEnvelope(fakeManagementError{status: 404, message: "404 Not Found: connection not found"})

		assert.Equal(t, "not_found", envelope.Error.Code)
		assert.Equal(t, "404 Not Found: connection not found", envelope.Error.Message)
		assert.Equal(t, 404, envelope.Error.Status)
		assert.Nil(t, envelope.Error.Details)
	})

	t.Run("omits status for non-API errors", func(t *testing.T) {
		envelope := buildErrorEnvelope(usageError{errors.New("unknown flag --foo")})

		assert.Equal(t, "usage", envelope.Error.Code)
		assert.Equal(t, "unknown flag --foo", envelope.Error.Message)
		assert.Zero(t, envelope.Error.Status)
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

// detailedError carries structured details for the envelope's "details" field.
type detailedError struct {
	details json.RawMessage
}

func (e detailedError) Error() string                 { return "validation failed" }
func (e detailedError) ErrorDetails() json.RawMessage { return e.details }

func TestBuildErrorEnvelopeDetails(t *testing.T) {
	details := json.RawMessage(`{"field":"name","reason":"required"}`)
	envelope := buildErrorEnvelope(detailedError{details: details})

	assert.Equal(t, "unknown", envelope.Error.Code)
	assert.JSONEq(t, string(details), string(envelope.Error.Details))
}

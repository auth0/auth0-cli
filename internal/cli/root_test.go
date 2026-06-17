package cli

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
)

type testManagementError struct {
	message string
	status  int
}

func (m testManagementError) Error() string {
	return m.message
}

func (m testManagementError) Status() int {
	return m.status
}

func TestCommandRequiresAuthentication(t *testing.T) {
	var testCases = []struct {
		givenCommand                    string
		expectedToRequireAuthentication bool
	}{
		{"auth0 user list", true},
		{"auth0 user create", true},
		{"auth0 api", true},
		{"auth0 apps list", true},
		{"auth0 apps create", true},
		{"auth0 orgs members list", true},
		{"auth0 completion", false},
		{"auth0 help", false},
		{"auth0 login", false},
		{"auth0 logout", false},
		{"auth0 tenants use", false},
		{"auth0 tenants list", false},
	}

	for index, testCase := range testCases {
		t.Run(fmt.Sprintf("TestCase #%d Command: %s", index, testCase.givenCommand), func(t *testing.T) {
			actualAuth := commandRequiresAuthentication(testCase.givenCommand)
			assert.Equal(t, testCase.expectedToRequireAuthentication, actualAuth)
		})
	}
}

func TestClassifyCommandFailure(t *testing.T) {
	t.Run("classifies 401 and 403 management errors as auth", func(t *testing.T) {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			props := classifyCommandFailure(testManagementError{message: "auth error", status: status})
			assert.Equal(t, "false", props["success"])
			assert.Equal(t, "auth", props["error_class"])
		}
	})

	t.Run("classifies 400 and 422 management errors as validation", func(t *testing.T) {
		for _, status := range []int{http.StatusBadRequest, http.StatusUnprocessableEntity} {
			props := classifyCommandFailure(testManagementError{message: "validation error", status: status})
			assert.Equal(t, "validation", props["error_class"])
		}
	})

	t.Run("classifies 404 as not_found", func(t *testing.T) {
		props := classifyCommandFailure(testManagementError{message: "not found", status: http.StatusNotFound})
		assert.Equal(t, "not_found", props["error_class"])
	})

	t.Run("classifies 429 as rate_limit", func(t *testing.T) {
		props := classifyCommandFailure(testManagementError{message: "rate limited", status: http.StatusTooManyRequests})
		assert.Equal(t, "rate_limit", props["error_class"])
	})

	t.Run("classifies 5xx as api", func(t *testing.T) {
		wrapped := fmt.Errorf("wrapped: %w", testManagementError{message: "server error", status: http.StatusServiceUnavailable})
		props := classifyCommandFailure(wrapped)
		assert.Equal(t, "api", props["error_class"])
	})

	t.Run("classifies non-management errors as unknown", func(t *testing.T) {
		props := classifyCommandFailure(errors.New("boom"))
		assert.Equal(t, "false", props["success"])
		assert.Equal(t, "unknown", props["error_class"])
	})

	t.Run("classifies auth config errors as auth", func(t *testing.T) {
		for _, err := range []error{
			config.ErrInvalidToken,
			config.ErrMalformedToken,
			config.ErrTokenMissingRequiredScopes{MissingScopes: []string{"read:users"}},
		} {
			props := classifyCommandFailure(err)
			assert.Equal(t, "auth", props["error_class"])
		}
	})
}

func TestTestManagementErrorSatisfiesManagementError(t *testing.T) {
	var _ management.Error = testManagementError{}
}

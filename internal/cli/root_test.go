package cli

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
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

func TestOutputFormatForTracking(t *testing.T) {
	t.Run("returns table for nil renderer", func(t *testing.T) {
		assert.Equal(t, "table", outputFormatForTracking(nil))
	})

	t.Run("returns table for default renderer format", func(t *testing.T) {
		renderer := &display.Renderer{}
		assert.Equal(t, "table", outputFormatForTracking(renderer))
	})

	t.Run("returns configured renderer format", func(t *testing.T) {
		renderer := &display.Renderer{Format: display.OutputFormatJSONCompact}
		assert.Equal(t, "json-compact", outputFormatForTracking(renderer))
	})
}

func TestIsCIEnvironment(t *testing.T) {
	t.Run("returns false when no CI vars are set", func(t *testing.T) {
		assert.False(t, isCIEnvironment(func(string) string { return "" }))
	})

	t.Run("returns true when CI var is truthy", func(t *testing.T) {
		getEnv := func(k string) string {
			if k == "CI" {
				return "true"
			}
			return ""
		}
		assert.True(t, isCIEnvironment(getEnv))
	})

	t.Run("returns false when CI var is explicit false", func(t *testing.T) {
		getEnv := func(k string) string {
			if k == "CI" {
				return "false"
			}
			return ""
		}
		assert.False(t, isCIEnvironment(getEnv))
	})

	t.Run("returns true for other known CI providers", func(t *testing.T) {
		getEnv := func(k string) string {
			if k == "GITHUB_ACTIONS" {
				return "1"
			}
			return ""
		}
		assert.True(t, isCIEnvironment(getEnv))
	})
}

func TestIsAPICommand(t *testing.T) {
	tests := []struct {
		name        string
		commandPath string
		expected    bool
	}{
		{"api command", "auth0 api", true},
		{"root command", "auth0", false},
		{"apis command is not the api command", "auth0 apis list", false},
		{"empty command path", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isAPICommand(test.commandPath))
		})
	}
}

func TestMergeProperties(t *testing.T) {
	base := map[string]string{"interactive": "true", "success": "true"}
	override := map[string]string{"success": "false", "error_class": "auth"}
	merged := mergeProperties(base, override)

	assert.Equal(t, "true", merged["interactive"])
	assert.Equal(t, "false", merged["success"])
	assert.Equal(t, "auth", merged["error_class"])
}

func TestResolveAgentMode(t *testing.T) {
	// The newCmd helper builds a command carrying the --agent-mode flag,
	// optionally marking it as explicitly set to a given value.
	newCmd := func(flagSet bool, flagValue bool) *cobra.Command {
		cmd := &cobra.Command{Use: "list"}
		cmd.Flags().Bool("agent-mode", false, "")
		if flagSet {
			_ = cmd.Flags().Set("agent-mode", strconv.FormatBool(flagValue))
		}
		return cmd
	}

	// Precedence: flag > env > detection.
	tests := []struct {
		name      string
		env       string
		flagSet   bool
		flagValue bool
		detected  string
		expected  bool
	}{
		{name: "flag false wins over env true", env: "true", flagSet: true, flagValue: false, detected: "claude-code", expected: false},
		{name: "flag true wins over env false", env: "false", flagSet: true, flagValue: true, detected: "human", expected: true},
		{name: "env truthy enables when flag unset", env: "1", detected: "human", expected: true},
		{name: "detected agent enables when flag and env unset", env: "", detected: "claude-code", expected: true},
		{name: "human is not agent mode", env: "", detected: "human", expected: false},
		{name: "unknown is not agent mode", env: "", detected: "unknown", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(agentModeEnvVar, test.env)
			cmd := newCmd(test.flagSet, test.flagValue)
			assert.Equal(t, test.expected, resolveAgentMode(cmd, test.detected))
		})
	}
}

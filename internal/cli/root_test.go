package cli

import (
	"errors"
	"fmt"
	"net/http"
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

func (m testManagementError) Code() string {
	return ""
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
		{"auth0 __complete", false},
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

func TestEnforceUnknownSubcommand(t *testing.T) {
	newTree := func() *cobra.Command {
		root := &cobra.Command{Use: "auth0"}
		group := &cobra.Command{Use: "actions"}
		leaf := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
		group.AddCommand(leaf)
		root.AddCommand(group)
		enforceUnknownSubcommand(root)
		return root
	}

	t.Run("namespace rejects an unknown subcommand as a usage error", func(t *testing.T) {
		group, _, err := newTree().Find([]string{"actions"})
		assert.NoError(t, err)

		err = group.Args(group, []string{"lst"})
		assert.Error(t, err)

		var usageErr usageError
		assert.True(t, errors.As(err, &usageErr))
		// The class is still "usage" for the JSON envelope, but every failure
		// collapses to the generic exit code for backwards compatibility.
		assert.Equal(t, "usage", errorClass(err))
		assert.Equal(t, exitGeneric, exitCodeForError(err))
	})

	t.Run("namespace accepts no args and prints help", func(t *testing.T) {
		group, _, err := newTree().Find([]string{"actions"})
		assert.NoError(t, err)
		assert.NoError(t, group.Args(group, []string{}))
		assert.True(t, group.Runnable())
	})

	t.Run("root rejects an unknown top-level command as a usage error", func(t *testing.T) {
		root := newTree()
		assert.Error(t, root.Args(root, []string{"bogus"}))
		assert.Equal(t, exitGeneric, exitCodeForError(root.Args(root, []string{"bogus"})))
	})

	t.Run("does not override a runnable leaf command", func(t *testing.T) {
		leaf, _, err := newTree().Find([]string{"actions", "list"})
		assert.NoError(t, err)
		assert.Nil(t, leaf.Args)
	})

	t.Run("namespace still rejects unknown flags", func(t *testing.T) {
		group, _, err := newTree().Find([]string{"actions"})
		assert.NoError(t, err)
		// Whitelisting unknown flags would let `auth0 actions --bogus` (and even
		// `auth0 actions --bogus list`, which swallows the subcommand) print help
		// and exit 0. The namespace must keep rejecting unknown flags so they
		// surface as a usage error with a non-zero exit.
		assert.False(t, group.FParseErrWhitelist.UnknownFlags)
	})
}

func TestWrapFlagError(t *testing.T) {
	// Drive real command execution so wrapFlagError sees exactly what pflag
	// leaves behind (the leftover positional, if any) at flag-error time.
	newTree := func() *cobra.Command {
		root := &cobra.Command{Use: "auth0", SilenceUsage: true, SilenceErrors: true}
		root.PersistentFlags().Bool("debug", false, "")
		group := &cobra.Command{Use: "actions"}
		leaf := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
		group.AddCommand(leaf)
		root.AddCommand(group)
		enforceUnknownSubcommand(root)
		root.SetFlagErrorFunc(wrapFlagError)
		return root
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "mistyped subcommand plus unknown flag reports both",
			args:    []string{"actions", "lst", "--bogus"},
			wantErr: `unknown command "lst" for "auth0 actions" (also: unknown flag: --bogus)`,
		},
		{
			name:    "flag before the positional reports only the flag",
			args:    []string{"actions", "--bogus", "lst"},
			wantErr: "unknown flag: --bogus",
		},
		{
			name:    "unknown flag with no positional reports only the flag",
			args:    []string{"actions", "--bogus"},
			wantErr: "unknown flag: --bogus",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newTree()
			root.SetArgs(test.args)
			err := root.Execute()
			assert.EqualError(t, err, test.wantErr)

			var usageErr usageError
			assert.True(t, errors.As(err, &usageErr))
			assert.Equal(t, "usage", errorClass(err))
			assert.Equal(t, exitGeneric, exitCodeForError(err))
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

	t.Run("emits the finer error_reason alongside error_class", func(t *testing.T) {
		props := classifyCommandFailure(config.ErrInvalidToken)
		assert.Equal(t, "auth", props["error_class"])
		assert.Equal(t, "session_expired", props["error_reason"])
	})
}

func TestClassifyRequiredFlagError(t *testing.T) {
	t.Run("wraps cobra's missing-required-flag error as usage/required_flag", func(t *testing.T) {
		err := classifyRequiredFlagError(errors.New(`required flag(s) "client-id" not set`))

		var usageErr usageError
		assert.True(t, errors.As(err, &usageErr))
		assert.Equal(t, "usage", errorClass(err))
		assert.Equal(t, "required_flag", errorReason(err))
	})

	t.Run("passes a nil error through", func(t *testing.T) {
		assert.NoError(t, classifyRequiredFlagError(nil))
	})

	t.Run("leaves an unrelated error unwrapped", func(t *testing.T) {
		err := errors.New("something else")
		assert.Same(t, err, classifyRequiredFlagError(err))
	})

	t.Run("does not double-wrap an already classified usage error", func(t *testing.T) {
		original := usageError{err: errors.New("unknown flag: --bogus")}
		assert.Equal(t, original, classifyRequiredFlagError(original))
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

func TestCommandTrackingProperties(t *testing.T) {
	t.Run("includes tenant domain when authenticated", func(t *testing.T) {
		c := &cli{tenant: "example.us.auth0.com", renderer: &display.Renderer{}}
		props := commandTrackingProperties(c)
		assert.Equal(t, "example.us.auth0.com", props["tenant"])
	})

	t.Run("includes empty tenant when unauthenticated", func(t *testing.T) {
		c := &cli{tenant: "", renderer: &display.Renderer{}}
		props := commandTrackingProperties(c)
		assert.Equal(t, "", props["tenant"])
	})
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
	// Precedence: an explicit --agent-mode flag in args > AUTH0_AGENT_MODE > detection.
	tests := []struct {
		name     string
		env      string
		args     []string
		detected string
		expected bool
	}{
		{name: "bare flag wins over env false", args: []string{"--agent-mode"}, env: "false", detected: "human", expected: true},
		{name: "flag=false wins over env true and detection", args: []string{"--agent-mode=false"}, env: "true", detected: "claude-code", expected: false},
		{name: "flag=true enables", args: []string{"--agent-mode=true"}, detected: "human", expected: true},
		{name: "unparseable flag falls through to env", args: []string{"--agent-mode=maybe"}, env: "1", detected: "human", expected: true},
		{name: "env truthy when no flag", env: "1", detected: "human", expected: true},
		{name: "env false disables even when detected", env: "false", detected: "claude-code", expected: false},
		{name: "detected agent when no flag or env", detected: "claude-code", expected: true},
		{name: "human is not agent mode", detected: "human", expected: false},
		{name: "unknown is not agent mode", detected: "unknown", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(agentModeEnvVar, test.env)
			assert.Equal(t, test.expected, resolveAgentMode(test.detected, test.args))
		})
	}
}

func TestApplyAgentModeDefaults(t *testing.T) {
	t.Run("disabled sets nothing", func(t *testing.T) {
		c := &cli{agentMode: false}
		applyAgentModeDefaults(c, &cobra.Command{Use: "list"})
		assert.False(t, c.json)
		assert.False(t, c.noInput)
		assert.False(t, c.noColor)
	})

	t.Run("enabled sets json/no-input/no-color", func(t *testing.T) {
		c := &cli{agentMode: true}
		applyAgentModeDefaults(c, &cobra.Command{Use: "list"})
		assert.True(t, c.json)
		assert.True(t, c.noInput)
		assert.True(t, c.noColor)
	})

	t.Run("explicit output flag is not overridden", func(t *testing.T) {
		cmd := &cobra.Command{Use: "list"}
		cmd.Flags().Bool("csv", false, "")
		_ = cmd.Flags().Set("csv", "true")

		c := &cli{agentMode: true}
		applyAgentModeDefaults(c, cmd)
		assert.False(t, c.json) // JSON not forced because --csv was explicit.
	})
}

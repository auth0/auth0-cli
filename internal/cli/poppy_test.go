package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// newPoppyTestRoot builds the real command tree without the authentication
// gate, so a command can be executed without a login.
func newPoppyTestRoot() *cobra.Command {
	testCLI := &cli{renderer: display.NewRenderer()}
	root := buildRootCmd(testCLI)
	root.PersistentPreRunE = nil
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	addSubCommands(root, testCLI)

	return root
}

func TestPoppyCommands(t *testing.T) {
	tests := []struct {
		name    string
		path    []string
		aliases []string
		flags   []string
	}{
		{
			name: "poppy",
			path: []string{"poppy"},
		},
		{
			name: "poppy setup",
			path: []string{"poppy", "setup"},
			flags: []string{
				"company-domain", "api", "api-url", "app", "scope", "hint", "name", "login-uri", "out",
				"dry-run", "yes", "skip-doctor", "guest-api-access", "guest-scopes", "guest-all-scopes", "force-config", "json", "json-compact",
			},
		},
		{
			name: "poppy config",
			path: []string{"poppy", "config"},
			flags: []string{
				"company-domain", "api", "api-url", "app", "hint", "name", "out",
				"force-config", "json", "json-compact",
			},
		},
		{
			name:  "poppy doctor",
			path:  []string{"poppy", "doctor"},
			flags: []string{"config", "json", "json-compact"},
		},
		{
			name: "poppy agents",
			path: []string{"poppy", "agents"},
		},
		{
			name: "poppy agents register",
			path: []string{"poppy", "agents", "register"},
			flags: []string{
				"api", "name", "out", "yes", "scopes", "all-scopes", "require-dpop", "json", "json-compact",
			},
		},
		{
			name:  "poppy agents list",
			path:  []string{"poppy", "agents", "list"},
			flags: []string{"all", "json", "json-compact"},
		},
		{
			name:  "poppy agents show",
			path:  []string{"poppy", "agents", "show"},
			flags: []string{"json", "json-compact"},
		},
		{
			name:    "poppy agents remove",
			path:    []string{"poppy", "agents", "remove"},
			aliases: []string{"unregister", "rm"},
			flags:   []string{"force", "dry-run", "remove-all-grants", "json", "json-compact"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newPoppyTestRoot()

			cmd, _, err := root.Find(test.path)
			require.NoError(t, err)
			require.Equal(t, test.path[len(test.path)-1], cmd.Name())
			assert.ElementsMatch(t, test.aliases, cmd.Aliases)
			assert.Contains(t, cmd.Long, "(Experimental)")
			assert.Contains(t, cmd.Long, poppyExperimentalWarning)

			var flags []string
			cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { flags = append(flags, f.Name) })
			assert.ElementsMatch(t, test.flags, flags)

			for _, alias := range test.aliases {
				byAlias, _, err := root.Find(append(append([]string{}, test.path[:len(test.path)-1]...), alias))
				require.NoError(t, err)
				assert.Same(t, cmd, byAlias)
			}
		})
	}

	t.Run("auth0 test poppy is not registered", func(t *testing.T) {
		testCmd, _, err := newPoppyTestRoot().Find([]string{"test"})
		require.NoError(t, err)

		for _, sub := range testCmd.Commands() {
			assert.NotEqual(t, "poppy", sub.Name())
		}
	})

	t.Run("agents register rejects --scopes with --all-scopes", func(t *testing.T) {
		root := newPoppyTestRoot()
		root.SetArgs([]string{"poppy", "agents", "register", "--scopes", "orders:read", "--all-scopes"})

		err := root.Execute()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "scopes")
		assert.Contains(t, err.Error(), "all-scopes")
	})
}

func TestPoppyDoctorSkipsAuthGate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tests := []struct {
		path    []string
		wantErr bool
	}{
		{path: []string{"poppy", "doctor"}},
		{path: []string{"poppy", "setup"}, wantErr: true},
		{path: []string{"poppy", "agents", "list"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			testCLI := &cli{renderer: display.NewRenderer()}
			root := buildRootCmd(testCLI)
			addSubCommands(root, testCLI)
			cmd, _, err := root.Find(test.path)
			require.NoError(t, err)

			err = root.PersistentPreRunE(cmd, nil)

			if test.wantErr {
				require.Error(t, err, "only doctor runs without a login")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPoppyDoctorDomain(t *testing.T) {
	dir := t.TempDir()
	writeConfig := func(t *testing.T, name, domain string) string {
		t.Helper()
		data, err := json.Marshal(config.Config{Organization: config.Organization{Domain: domain}})
		require.NoError(t, err)
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		return path
	}
	broken := filepath.Join(dir, "broken.json")
	require.NoError(t, os.WriteFile(broken, []byte("{"), 0o600))

	tests := []struct {
		name       string
		args       []string
		configFile string
		want       string
		wantErr    string
	}{
		{name: "argument", args: []string{"Shop.Example"}, configFile: "missing.json", want: "shop.example"},
		{name: "argument as URL", args: []string{"https://shop.example/"}, want: "shop.example"},
		{name: "argument wins over the config file", args: []string{"a.example"}, configFile: writeConfig(t, "a.json", "b.example"), want: "a.example"},
		{name: "argument with a path", args: []string{"shop.example/orders"}, wantErr: "path"},
		{name: "argument with a port", args: []string{"shop.example:443"}, wantErr: "port"},
		{name: "config file", configFile: writeConfig(t, "c.json", "shop.example"), want: "shop.example"},
		{name: "no argument and no config file", configFile: filepath.Join(dir, "missing.json"), wantErr: "pass a domain"},
		{name: "config file without a domain", configFile: writeConfig(t, "d.json", ""), wantErr: "organization.domain"},
		{name: "unreadable config file", configFile: broken, wantErr: "failed to parse"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := poppyDoctorDomain(test.args, test.configFile)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

// newDoctorTestOptions points every host name at a TLS server that answers 404.
func newDoctorTestOptions(t *testing.T) doctor.Options {
	t.Helper()

	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}

	return doctor.Options{HTTPClient: &http.Client{Transport: transport}}
}

func TestRunPoppyDoctor(t *testing.T) {
	tests := []struct {
		name   string
		format display.OutputFormat
		check  func(t *testing.T, out string)
	}{
		{
			name: "text",
			check: func(t *testing.T, out string) {
				assert.Contains(t, out, "FAIL https://127.0.0.1/.well-known/poppy.json returned HTTP 404")
				assert.Contains(t, out, "- skipped: poppy.json is not available")
				assert.Contains(t, out, "failed")
			},
		},
		{
			name:   "json",
			format: display.OutputFormatJSON,
			check: func(t *testing.T, out string) {
				var report doctor.Report
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				assert.Equal(t, "127.0.0.1", report.Domain)
				assert.Equal(t, 1, report.ExitCode())
			},
		},
		{
			name:   "json compact",
			format: display.OutputFormatJSONCompact,
			check: func(t *testing.T, out string) {
				assert.Equal(t, 1, strings.Count(out, "\n"), "one line")
				var report doctor.Report
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				assert.Equal(t, "127.0.0.1", report.Domain)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			testCLI := &cli{renderer: &display.Renderer{ResultWriter: &out, MessageWriter: io.Discard, Format: test.format}}

			err := runPoppyDoctor(t.Context(), testCLI, "127.0.0.1", newDoctorTestOptions(t))

			require.ErrorIs(t, err, errPoppyDoctorFailed)
			test.check(t, out.String())
		})
	}
}

// poppySessionToken is a well-formed JWT: only its shape is checked.
const poppySessionToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q"

// saveSession writes a saved login to the config file under a new home directory.
func saveSession(t *testing.T, defaultTenant string, tenants map[string]any) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(authModeEnvVar, "")
	keyring.MockInit()

	saved, err := json.Marshal(map[string]any{"default_tenant": defaultTenant, "tenants": tenants})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "auth0"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "auth0", "config.json"), saved, 0o600))
}

func TestOptionalPoppyTenant(t *testing.T) {
	const domain = "tenant.us.auth0.com"

	valid := func(extra map[string]any) map[string]any {
		tenant := map[string]any{
			"domain":       domain,
			"access_token": poppySessionToken,
			"scopes":       auth.RequiredScopes,
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		maps.Copy(tenant, extra)

		return map[string]any{domain: tenant}
	}

	tests := []struct {
		name          string
		setup         func(t *testing.T)
		tenant        string
		wantTenant    bool
		wantErr       string
		wantTenantSet string
	}{
		{
			name:  "no saved login",
			setup: func(t *testing.T) { t.Setenv("HOME", t.TempDir()); t.Setenv(authModeEnvVar, "") },
		},
		{
			name:  "a usable saved login",
			setup: func(t *testing.T) { saveSession(t, domain, valid(nil)) },

			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name:   "the tenant flag picks another saved login",
			setup:  func(t *testing.T) { saveSession(t, "other.us.auth0.com", valid(nil)) },
			tenant: domain,

			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name:  "no default tenant",
			setup: func(t *testing.T) { saveSession(t, "", valid(nil)) },
		},
		{
			name:   "the tenant flag names a tenant that is not saved",
			setup:  func(t *testing.T) { saveSession(t, domain, valid(nil)) },
			tenant: "missing.us.auth0.com",
		},
		{
			name: "an expired device login, which a command would answer with a device-code login",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}))
			},
		},
		{
			name: "an expired client-credentials login, which a command would refresh",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"client_id": "client", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}))
			},
		},
		{
			name:  "a device login without the required scopes, which a command would answer with a login",
			setup: func(t *testing.T) { saveSession(t, domain, valid(map[string]any{"scopes": []string{"openid"}})) },
		},
		{
			name:  "a login with a corrupted token",
			setup: func(t *testing.T) { saveSession(t, domain, valid(map[string]any{"access_token": "not-a-token"})) },
		},
		{
			name: "a login whose token cannot be read",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"access_token": ""}))
			},
		},
		{
			name: "environment credentials",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, domain)
				t.Setenv(envAPIToken, "token")
			},
			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name: "environment credentials that are incomplete",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, "")
			},
			wantErr: "requires",
		},
		{
			name: "an unknown auth mode",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, "evn")
			},
			wantErr: "unrecognized",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)

			var messages bytes.Buffer
			testCLI := &cli{renderer: &display.Renderer{MessageWriter: &messages, ResultWriter: io.Discard}, tenant: test.tenant}

			got, err := testCLI.optionalPoppyTenant(t.Context())

			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Empty(t, messages.String(), "nothing is printed, asked or warned about")

			if !test.wantTenant {
				assert.Nil(t, got)
				assert.Nil(t, testCLI.apiv3, "no client is built")
				return
			}

			require.NotNil(t, got)
			assert.NotNil(t, got.State)
			assert.NotNil(t, got.Agents)
			assert.NotNil(t, got.Policies)
			assert.Equal(t, test.wantTenantSet, testCLI.tenant)
		})
	}
}

func TestOptionalPoppyTenantDoesNotChangeTheConfig(t *testing.T) {
	// A saved login without a default tenant is repaired by a command that needs
	// a login, which writes the config. The doctor leaves the file alone.
	saveSession(t, "", map[string]any{"a.us.auth0.com": map[string]any{"domain": "a.us.auth0.com", "access_token": poppySessionToken}})

	path := filepath.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	got, err := (&cli{renderer: display.NewRenderer()}).optionalPoppyTenant(t.Context())

	require.NoError(t, err)
	assert.Nil(t, got)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

func TestPoppyDoctorCommandWithoutLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(authModeEnvVar, "")

	var out bytes.Buffer
	testCLI := &cli{
		renderer:    &display.Renderer{ResultWriter: &out, MessageWriter: io.Discard},
		poppyDoctor: newDoctorTestOptions(t),
	}

	root := buildRootCmd(testCLI)
	root.PersistentPreRunE = nil
	root.SilenceErrors, root.SilenceUsage = true, true
	addSubCommands(root, testCLI)
	root.SetArgs([]string{"poppy", "doctor", "shop.example", "--config", filepath.Join(t.TempDir(), "missing.json")})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, errPoppyDoctorFailed, "the website cannot be reached")
	text := out.String()
	assert.Contains(t, text, "FAIL https://shop.example/.well-known/poppy.json could not be fetched")
	assert.Equal(t, 1, strings.Count(text, "skipped (6 checks): no usable tenant login; run auth0 login to check the tenant"), "the same reason is said once")
	assert.Nil(t, testCLI.apiv3, "no client is built")
}

// poppyPromptMode is one way to run a Poppy command: where it runs and the
// flags it gets.
type poppyPromptMode struct {
	terminal  bool
	agentMode bool
	flags     []string
}

var (
	modeTerminal        = poppyPromptMode{terminal: true}
	modeTerminalJSON    = poppyPromptMode{terminal: true, flags: []string{"--json"}}
	modeTerminalNoInput = poppyPromptMode{terminal: true, flags: []string{"--no-input"}}
	modePiped           = poppyPromptMode{}
	modePipedJSON       = poppyPromptMode{flags: []string{"--json"}}
	modeAgent           = poppyPromptMode{terminal: true, agentMode: true}
	modeAgentPrompting  = poppyPromptMode{terminal: true, agentMode: true, flags: []string{"--no-input=false"}}
)

// poppyPromptRow is what a command is expected to do in a mode.
type poppyPromptRow struct {
	name string
	mode poppyPromptMode
	// The questions asked, in order.
	asked []string
	// Text on stdout before any JSON document, empty when there is none.
	text string
	// Text on stderr, empty when it is not checked.
	message string
	// Whether stdout ends with a JSON document.
	json bool
	// The reason of the error, empty when there is none.
	reason string
	// Text of the error message.
	errText string
}

// poppyPromptRun is what a command did in a mode.
type poppyPromptRun struct {
	asked  []string
	out    string
	errOut string
	err    error
}

// installPoppyAsker makes the Poppy commands behave as if run in the terminal
// of the mode, or without one. A person answers yes to every question, except
// the one that offers to set the hints of the scopes, which is turned down. The
// questions are kept in the order they were asked.
func installPoppyAsker(t *testing.T, mode poppyPromptMode) *[]string {
	t.Helper()

	var asked []string

	previous := poppyAsk
	t.Cleanup(func() { poppyAsk = previous })

	poppyAsk.canPrompt = func(cmd *cobra.Command) bool {
		noInput, err := cmd.Root().Flags().GetBool("no-input")

		return err == nil && mode.terminal && !noInput
	}
	poppyAsk.text = func(_ *poppyScreen, label, _ string) (string, error) {
		asked = append(asked, "text "+label)

		return "Reads the orders of the signed-in user.", nil
	}
	poppyAsk.confirm = func(_ *poppyScreen, message string, _ bool) (bool, error) {
		if strings.HasPrefix(message, "Set ") {
			asked = append(asked, "hints")

			return false, nil
		}

		asked = append(asked, "confirm")

		return true, nil
	}
	poppyAsk.guest = func(*poppyScreen) poppyGuestPrompts {
		return poppyGuestPrompts{
			allow: func() (bool, error) {
				asked = append(asked, "guest access")

				return true, nil
			},
			all: func() (bool, error) {
				asked = append(asked, "guest scopes")

				return true, nil
			},
			note: func(string, ...any) {},
		}
	}

	return &asked
}

// newTestPoppyScreen starts a screen that writes to a buffer, as if it were a
// terminal 80 cells wide when erase is set, and asks with the asker given. An
// interrupt ends the command with the error itself instead of the process.
func newTestPoppyScreen(t *testing.T, asker poppyAsker, erase bool) (*poppyScreen, *bytes.Buffer) {
	t.Helper()

	out := &bytes.Buffer{}
	renderer := &display.Renderer{ResultWriter: out, MessageWriter: io.Discard}

	if asker.interrupt == nil {
		asker.interrupt = func(err error) error { return err }
	}

	return &poppyScreen{
		PoppyScreen: renderer.NewPoppyScreen("Poppy setup", "mock.auth0.test", display.PoppyLayout{Width: 80, Unicode: true}, erase),
		asker:       asker,
		erase:       erase,
	}, out
}

// splitPoppyJSON splits the output at the JSON document it ends with, which
// starts a line. The document is empty when the output does not end with one.
func splitPoppyJSON(out string) (text, document string) {
	for start := 0; start < len(out); start++ {
		if out[start] == '{' && (start == 0 || out[start-1] == '\n') && json.Valid([]byte(out[start:])) {
			return out[:start], out[start:]
		}
	}

	return out, ""
}

func assertPoppyPromptRun(t *testing.T, row poppyPromptRow, run poppyPromptRun) {
	t.Helper()

	text, document := splitPoppyJSON(run.out)

	assert.Equal(t, row.asked, run.asked, "questions asked")
	assert.Equal(t, row.json, document != "", "JSON document on stdout:\n%s", run.out)

	if row.text == "" {
		assert.Empty(t, text, "no human text on stdout")
	} else {
		assert.Contains(t, text, row.text)
	}

	assert.Contains(t, run.errOut, row.message)

	if row.reason == "" {
		require.NoError(t, run.err)

		return
	}

	require.Error(t, run.err)
	assert.Equal(t, row.reason, errorReason(run.err))
	assert.Contains(t, run.err.Error(), row.errText)
}

const (
	poppySetupTenant = "tenant.us.auth0.com"
	poppySetupAppID  = "W1x9aBcD2eFgH3iJ"
)

// poppyTenantState is the part of a tenant that the Poppy setup commands read.
type poppyTenantState struct {
	settings *managementv3.GetTenantSettingsResponseContent
	domains  []*management.CustomDomain
	server   *managementv3.ResourceServer
	app      *managementv3.GetClientResponseContent
	guest    bool
}

// freshPoppyTenant is a tenant that has never been set up for Poppy.
func freshPoppyTenant() poppyTenantState {
	return poppyTenantState{
		settings: &managementv3.GetTenantSettingsResponseContent{
			ResourceParameterProfile: testPtr(managementv3.TenantSettingsResourceParameterProfileAudience),
			FriendlyName:             testPtr("Shop"),
		},
		server: &managementv3.ResourceServer{
			ID:         testPtr("rs_shop"),
			Identifier: testPtr(poppyTestAPI),
			Name:       testPtr("Shop API"),
			Scopes: []*managementv3.ResourceServerScope{
				{Value: "orders:read", Description: testPtr("See your orders and their status. Read only.")},
				{Value: "checkout", Description: testPtr("Place orders and pay with your saved card. Can't be undone.")},
			},
		},
		app: &managementv3.GetClientResponseContent{
			ClientID:         testPtr(poppySetupAppID),
			Name:             testPtr("Shop Web"),
			IsFirstParty:     testPtr(true),
			InitiateLoginURI: testPtr("https://shop.example/login"),
		},
	}
}

// configuredPoppyTenant is a tenant that setup has already been run against.
func configuredPoppyTenant() poppyTenantState {
	state := freshPoppyTenant()

	state.settings = &managementv3.GetTenantSettingsResponseContent{
		ClientIDMetadataDocumentSupported:          testPtr(true),
		AuthorizationResponseIssParameterSupported: testPtr(true),
		ResourceParameterProfile:                   testPtr(managementv3.TenantSettingsResourceParameterProfileCompatibility),
		DefaultAudience:                            testPtr(poppyTestAPI),
		FriendlyName:                               testPtr("Shop"),
		PersonalAgentProtocolCompanyDomains:        []string{"https://shop.example"},
	}
	state.domains = []*management.CustomDomain{{Domain: testPtr("auth.shop.example"), Status: testPtr("ready"), IsDefault: testPtr(true)}}
	state.server.Scopes = append(state.server.Scopes, &managementv3.ResourceServerScope{Value: setup.BrowserSignInScope, Description: testPtr(setup.BrowserSignInDescription)})
	state.server.AllowOfflineAccess = testPtr(true)
	state.server.UseForPersonalAgentProtocol = testPtr(true)
	state.server.AgentSubjectClaims = testPtr(managementv3.ResourceServerAgentSubjectClaimsResponseEnumAuth0V1)
	state.server.SubjectTypeAuthorization = &managementv3.ResourceServerSubjectTypeAuthorization{
		AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{
			Policy: testPtr(managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant),
		},
	}
	state.app.AllowPersonalAgentBrowserJoin = testPtr(true)
	state.guest = true

	return state
}

type poppySetupHarness struct {
	out      *bytes.Buffer
	messages *bytes.Buffer
	tenant   *mock.MockTenantSettingsAPIV3
	domains  *mock.MockCustomDomainAPI
	servers  *mock.MockResourceServerAPIV3
	clients  *mock.MockClientAPIV3
	grants   *mock.MockClientGrantAPIV3
	agents   *mock.MockAgentAPIV3
	policies *mock.MockGuardianPolicyAPIV3
	run      func(args ...string) error

	// Runs the command in agent mode when set.
	agentMode bool

	// The Management API writes made, in order, and the error that a write of a
	// kind (tenant, api, app or grant) returns.
	calls []string
	fail  map[string]error
	// The last guest grant created.
	grant *managementv3.CreateClientGrantRequestContent
}

// allowWrites accepts the Management API writes of setup and records them in
// calls.
func (h *poppySetupHarness) allowWrites() {
	record := func(kind string) error {
		h.calls = append(h.calls, kind)

		return h.fail[kind]
	}

	h.tenant.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *managementv3.UpdateTenantSettingsRequestContent, ...any) (*managementv3.UpdateTenantSettingsResponseContent, error) {
			return &managementv3.UpdateTenantSettingsResponseContent{}, record("tenant")
		}).AnyTimes()
	h.servers.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, string, *managementv3.UpdateResourceServerRequestContent, ...any) (*managementv3.UpdateResourceServerResponseContent, error) {
			return &managementv3.UpdateResourceServerResponseContent{}, record("api")
		}).AnyTimes()
	h.clients.EXPECT().Update(gomock.Any(), poppySetupAppID, gomock.Any()).DoAndReturn(
		func(context.Context, string, *managementv3.UpdateClientRequestContent, ...any) (*managementv3.UpdateClientResponseContent, error) {
			return &managementv3.UpdateClientResponseContent{}, record("app")
		}).AnyTimes()
	h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.CreateClientGrantRequestContent, _ ...any) (*managementv3.CreateClientGrantResponseContent, error) {
			h.grant = request

			return &managementv3.CreateClientGrantResponseContent{}, record("grant")
		}).AnyTimes()
}

// newPoppySetupHarness serves the tenant to the commands. The mocks have no
// write calls until allowWrites is called, so any attempt to change the tenant
// fails the test. The doctor checks that run at the end of setup see a website
// that answers 404.
func newPoppySetupHarness(t *testing.T, tenant poppyTenantState) *poppySetupHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &poppySetupHarness{
		out:      &bytes.Buffer{},
		messages: &bytes.Buffer{},
		tenant:   mock.NewMockTenantSettingsAPIV3(ctrl),
		domains:  mock.NewMockCustomDomainAPI(ctrl),
		servers:  mock.NewMockResourceServerAPIV3(ctrl),
		clients:  mock.NewMockClientAPIV3(ctrl),
		grants:   mock.NewMockClientGrantAPIV3(ctrl),
		agents:   mock.NewMockAgentAPIV3(ctrl),
		policies: mock.NewMockGuardianPolicyAPIV3(ctrl),
		fail:     map[string]error{},
	}

	// The doctor checks that run at the end of setup read the agents and the MFA policies too.
	h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil).AnyTimes()
	h.policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil).AnyTimes()

	h.tenant.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tenant.settings, nil).AnyTimes()
	h.domains.EXPECT().List(gomock.Any()).Return(tenant.domains, nil).AnyTimes()
	h.servers.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.ListResourceServerRequestParameters, _ ...any) (*auth0.ResourceServerPage, error) {
			for _, identifier := range request.Identifiers {
				if *identifier != tenant.server.GetIdentifier() {
					return testServerPage(), nil
				}
			}

			return testServerPage(tenant.server), nil
		}).AnyTimes()
	h.clients.EXPECT().Get(gomock.Any(), poppySetupAppID, gomock.Any()).Return(tenant.app, nil).AnyTimes()
	h.clients.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *managementv3.ListClientsRequestParameters, ...any) (*auth0.ClientListPage, error) {
			return testClientPage(
				&managementv3.Client{
					ClientID:             testPtr("client_muse"),
					ExternalClientID:     testPtr(poppyTestAgentURL),
					ExternalMetadataType: testPtr(managementv3.ClientExternalMetadataTypeEnumCimd),
				},
				&managementv3.Client{
					ClientID:                      tenant.app.ClientID,
					Name:                          tenant.app.Name,
					IsFirstParty:                  tenant.app.IsFirstParty,
					InitiateLoginURI:              tenant.app.InitiateLoginURI,
					AllowPersonalAgentBrowserJoin: tenant.app.AllowPersonalAgentBrowserJoin,
				},
			), nil
		}).AnyTimes()
	h.grants.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.ListClientGrantsRequestParameters, _ ...any) (*auth0.ClientGrantPage, error) {
			if request.ClientID == nil {
				return testGrantPage(&managementv3.ClientGrantResponseContent{ClientID: testPtr("client_muse")}), nil
			}

			if !tenant.guest {
				return testGrantPage(), nil
			}

			return testGrantPage(&managementv3.ClientGrantResponseContent{SubjectType: testPtr(managementv3.ClientGrantSubjectTypeEnumAnonymousUser)}), nil
		}).AnyTimes()

	renderer := display.NewRenderer()
	renderer.ResultWriter = h.out
	renderer.MessageWriter = h.messages

	testCLI := &cli{
		renderer:    renderer,
		poppyDoctor: newDoctorTestOptions(t),
		tenant:      poppySetupTenant,
		api:         &auth0.API{CustomDomain: h.domains},
		apiv3: &auth0.APIV3{
			TenantSettings:   h.tenant,
			ResourceServerV3: h.servers,
			ClientV3:         h.clients,
			ClientGrant:      h.grants,
			Agent:            h.agents,
			GuardianPolicy:   h.policies,
		},
	}

	h.run = func(args ...string) error {
		root := buildRootCmd(testCLI)
		root.PersistentFlags().BoolVar(&testCLI.noInput, "no-input", false, "Disable interactivity.")
		testCLI.agentMode = h.agentMode
		root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
			applyAgentModeDefaults(testCLI, cmd)
			testCLI.configureRenderer()

			return nil
		}
		root.SilenceErrors = true
		root.SilenceUsage = true
		addSubCommands(root, testCLI)
		root.SetArgs(append([]string{"poppy"}, args...))
		root.SetContext(context.Background())

		return root.Execute()
	}

	return h
}

// goldenPoppyConfig copies the config file that setup writes for the configured
// tenant into a new directory and returns its path.
func goldenPoppyConfig(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "poppy", "config", "testdata", "auth0-poppy.golden.json"))
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "auth0-poppy.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return path
}

func freshSetupArgs(t *testing.T, extra ...string) []string {
	t.Helper()

	return append([]string{
		"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
		"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json",
		"--out", filepath.Join(t.TempDir(), "auth0-poppy.json"),
	}, extra...)
}

func TestPoppySetupPlan(t *testing.T) {
	t.Run("dry run prints the plan and applies nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--dry-run", "--guest-api-access", "--guest-all-scopes")...)

		require.NoError(t, err)
		text := h.out.String()
		assert.Contains(t, text, "Poppy setup for shop.example")
		assert.Contains(t, text, "tenant "+poppySetupTenant)
		assert.Contains(t, text, "Dry run: nothing will be changed.")
		assert.Contains(t, text, "The tenant has no verified custom domain")
		assert.Contains(t, text, "affects all apps")
		assert.Contains(t, text, "CIMD support")
		assert.Contains(t, text, "off -> on")
		assert.Contains(t, text, "API "+poppyTestAPI+"\n")
		assert.Contains(t, text, "App Shop Web\n")
		assert.Contains(t, text, "browser join")
		assert.Contains(t, text, "Guest grant\n")
		assert.Contains(t, text, "File\n")
		assert.Contains(t, text, "To apply:  auth0 poppy setup --company-domain shop.example --api "+poppyTestAPI)
		assert.NotContains(t, text, "--dry-run", "the command that applies the plan is not a dry run")
	})

	t.Run("the guest grant is left out unless asked for", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(freshSetupArgs(t, "--dry-run")...))

		assert.NotContains(t, h.out.String(), "for guests")
	})

	t.Run("dry run as JSON", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(freshSetupArgs(t, "--dry-run", "--json")...))

		var plan setup.Plan
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &plan))
		assert.Equal(t, poppySetupTenant, plan.Tenant)
		require.NotEmpty(t, plan.Changes)
		assert.Equal(t, setup.KindTenant, plan.Changes[0].Kind)
		assert.False(t, plan.Empty())
	})

	t.Run("a tenant that is already set up has nothing to change", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())

		err := h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--guest-api-access", "--guest-all-scopes", "--out", goldenPoppyConfig(t))

		require.NoError(t, err, "no write call is set on the mocks")
		assert.Contains(t, h.out.String(), "Nothing to change")
	})
}

func TestPoppySetupInputErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		tenant     poppyTenantState
		wantReason string
		wantText   string
	}{
		{
			name:       "the company domain is required without a terminal",
			args:       []string{"setup", "--api", poppyTestAPI, "--app", poppySetupAppID},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_flag",
			wantText:   "--company-domain is required",
		},
		{
			name:       "the API is required without a terminal",
			args:       []string{"setup", "--company-domain", "shop.example", "--app", poppySetupAppID},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_flag",
			wantText:   "--api is required",
		},
		{
			name:       "the app is required without a terminal",
			args:       []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_flag",
			wantText:   "--app is required",
		},
		{
			name:       "the API does not exist",
			args:       []string{"setup", "--company-domain", "shop.example", "--api", "https://missing.example", "--app", poppySetupAppID},
			tenant:     freshPoppyTenant(),
			wantReason: "api_not_found",
			wantText:   "https://missing.example",
		},
		{
			name:       "the company domain must be a host",
			args:       []string{"setup", "--company-domain", "https://shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID},
			tenant:     freshPoppyTenant(),
			wantReason: "invalid_flag",
			wantText:   "--company-domain",
		},
		{
			name:       "the OpenAPI URL is required without a terminal",
			args:       []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "none.json")},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_flag",
			wantText:   "--api-url " + poppyTestAPI + "=<openapi url> is required",
		},
		{
			name: "the app must be first-party",
			args: []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID},
			tenant: func() poppyTenantState {
				state := freshPoppyTenant()
				state.app.IsFirstParty = testPtr(false)

				return state
			}(),
			wantReason: "app_not_first_party",
			wantText:   "Shop Web is not first-party",
		},
		{
			name: "a scope without a description",
			args: freshSetupArgs(t),
			tenant: func() poppyTenantState {
				state := freshPoppyTenant()
				state.server.Scopes[1].Description = nil

				return state
			}(),
			wantReason: "scope_description_required",
			wantText:   "checkout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, test.tenant)

			err := h.run(test.args...)

			reason, _ := decodeCLIError(t, err)
			assert.Equal(t, test.wantReason, reason)
			assert.Contains(t, err.Error(), test.wantText)
		})
	}
}

func TestPoppyConfig(t *testing.T) {
	t.Run("writes the file for a tenant that is set up", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "auth0-poppy.json")

		err := h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out)

		require.NoError(t, err)
		assert.Contains(t, h.out.String(), out+"  written")
		assert.Contains(t, h.out.String(), "Next\n  auth0 poppy doctor shop.example   to check the live site.")
		assert.Empty(t, h.messages.String(), "the result is on stdout")

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, "shop.example", cfg.Organization.Domain)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
		assert.Equal(t, "https://auth.shop.example/poppy/"+poppySetupAppID+"/browser-session", cfg.Web.BrowserSessionEndpoint)
		assert.Equal(t, "Shop API", cfg.APIs[0].Description)
		assert.Equal(t, poppyTestAPI+"/openapi.json", cfg.APIs[0].URL)
	})

	t.Run("a file that is already right is said to be unchanged", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "auth0-poppy.json")
		args := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", out}

		first := newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, first.run(args...))
		assert.Contains(t, first.out.String(), out+"  written")

		second := newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, second.run(args...))
		assert.Contains(t, second.out.String(), out+"  unchanged")
		assert.NotContains(t, second.out.String(), "written")
		assert.Contains(t, second.out.String(), "Next\n")
	})

	t.Run("the API is told by the file and the web app by the tenant", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		require.NoError(t, h.run("config", "--out", path))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.JSONEq(t, string(before), string(after), "rebuilding from a matching tenant changes nothing")
	})

	t.Run("--api alone is enough when the tenant has one app with the browser join", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "auth0-poppy.json")

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out))

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/poppy/"+poppySetupAppID+"/browser-session", cfg.Web.BrowserSessionEndpoint)
	})

	t.Run("prints the file as JSON", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)

		require.NoError(t, h.run("config", "--out", path, "--json"))

		var cfg config.Config
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &cfg))
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
		assert.Empty(t, h.messages.String())

		var doc map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &doc))
		assert.Contains(t, doc, "auth", "the JSON has the shape of the file")
		assert.NotContains(t, doc, "auth0", "the CLI adds no member of its own")
	})

	t.Run("members of an existing file that the CLI does not use are kept", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), `"apis": [`, `"feedback": {"endpoint": "https://shop.example/feedback"},
  "apis": [`, 1)), 0o600))

		require.NoError(t, h.run("config", "--out", path))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(after), `"endpoint": "https://shop.example/feedback"`)
	})

	t.Run("a file in the old layout is replaced by the new layout", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := filepath.Join(t.TempDir(), "auth0-poppy.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"protocol_version":"0.4","issuer":"https://auth.shop.example/","scopes":{},"apis":[],"web":{"client_id":"x"},"agents":[]}`), 0o600))

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", path))

		cfg, err := config.Read(path)
		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
	})

	t.Run("the company domain is picked from several without a terminal", func(t *testing.T) {
		tenant := configuredPoppyTenant()
		tenant.settings.PersonalAgentProtocolCompanyDomains = []string{"https://shop.example", "https://shop.test"}
		h := newPoppySetupHarness(t, tenant)

		err := h.run("config", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", filepath.Join(t.TempDir(), "none.json"))

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "missing_flag", reason)
		assert.Contains(t, err.Error(), "--company-domain is required: the tenant lists several company domains (shop.example, shop.test)")
	})
}

func TestPoppyConfigErrors(t *testing.T) {
	missingURL := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out"}

	tests := []struct {
		name       string
		tenant     poppyTenantState
		args       func(t *testing.T) []string
		wantReason string
		wantText   string
	}{
		{
			name:   "no API anywhere",
			tenant: configuredPoppyTenant(),
			args: func(t *testing.T) []string {
				return []string{"config", "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "none.json")}
			},
			wantReason: "api_not_found",
			wantText:   "There is no config file, so no API of the tenant is known.\n  FIX  Pass --api <identifier> to name the API, or create the file with auth0 poppy setup.",
		},
		{
			name: "no app anywhere",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.app.AllowPersonalAgentBrowserJoin = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return []string{"config", "--api", poppyTestAPI, "--out", filepath.Join(t.TempDir(), "none.json")}
			},
			wantReason: "app_not_found",
			wantText:   "No app of the tenant has allow_personal_agent_browser_join on.\n  FIX  Pass --app <client ID or name>",
		},
		{
			name:   "the OpenAPI URL is not stored anywhere",
			tenant: configuredPoppyTenant(),
			args: func(t *testing.T) []string {
				return append(missingURL, filepath.Join(t.TempDir(), "none.json"))
			},
			wantReason: "missing_config_values",
			wantText:   "apis[" + poppyTestAPI + "].url. Pass --api-url as <identifier>=<url>",
		},
		{
			name: "the company domain is not listed",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.settings.PersonalAgentProtocolCompanyDomains = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "tenant setting personal_agent_protocol_company_domains does not list https://shop.example",
		},
		{
			name: "the API does not use the protocol",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.server.UseForPersonalAgentProtocol = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "API " + poppyTestAPI + ": use_for_personal_agent_protocol is off",
		},
		{
			name: "the browser join is off",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.app.AllowPersonalAgentBrowserJoin = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "app Shop Web: allow_personal_agent_browser_join is off\n  FIX  Run: auth0 poppy setup --company-domain shop.example",
		},
		{
			name:   "the file belongs to another issuer",
			tenant: func() poppyTenantState { state := configuredPoppyTenant(); state.domains = nil; return state }(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "issuer_mismatch",
			wantText:   "Pass --force-config to overwrite it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, test.tenant)
			args := test.args(t)

			before := map[string][]byte{}
			if path := args[len(args)-1]; filepath.Ext(path) == ".json" {
				if data, err := os.ReadFile(path); err == nil {
					before[path] = data
				}
			}

			err := h.run(args...)

			reason, _ := decodeCLIError(t, err)
			assert.Equal(t, test.wantReason, reason)
			assert.Contains(t, err.Error(), test.wantText)
			assert.Empty(t, h.out.String())

			for path, data := range before {
				after, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				assert.Equal(t, data, after, "a refused command leaves the file alone")
			}
		})
	}

	t.Run("--force-config overwrites a file of another issuer", func(t *testing.T) {
		tenant := configuredPoppyTenant()
		tenant.domains = nil
		h := newPoppySetupHarness(t, tenant)
		path := goldenPoppyConfig(t)

		require.NoError(t, h.run(append(missingURL, path, "--force-config")...))

		cfg, err := config.Read(path)
		require.NoError(t, err)
		assert.Equal(t, "https://"+poppySetupTenant+"/", cfg.Issuer)
	})
}

func TestPoppySetupError(t *testing.T) {
	plain := errors.New("plain")

	tests := []struct {
		name       string
		err        error
		wantReason string
		wantText   string
	}{
		{
			name:       "an input error keeps its reason",
			err:        &setup.InputError{Message: "bad", Reason: "invalid_flag", Fix: "do this"},
			wantReason: "invalid_flag",
			wantText:   "bad\n  FIX  do this",
		},
		{
			name:       "an unready tenant",
			err:        &setup.NotReadyError{Problems: []string{"a"}, Fix: "b"},
			wantReason: "poppy_not_ready",
			wantText:   "the tenant is not set up for Poppy:\n  - a\n  FIX  b",
		},
		{
			name:       "missing values",
			err:        &config.MissingFieldsError{Fields: []string{"apis[x].url"}},
			wantReason: "missing_config_values",
			wantText:   "apis[x].url. Pass --api-url as <identifier>=<url>, or run auth0 poppy setup",
		},
		{
			name:       "a wrapped issuer mismatch",
			err:        errors.Join(config.ErrIssuerMismatch, errors.New("detail")),
			wantReason: "issuer_mismatch",
			wantText:   "Pass --force-config to overwrite it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := poppySetupError(test.err)

			assert.Equal(t, test.wantReason, errorReason(err))
			assert.Contains(t, err.Error(), test.wantText)
			assert.ErrorIs(t, err, test.err)
		})
	}

	t.Run("other errors pass through", func(t *testing.T) {
		assert.Same(t, plain, poppySetupError(plain))
		assert.NoError(t, poppySetupError(nil))
	})
}

// outOf returns the config file path in the arguments.
func outOf(args []string) string {
	return args[slices.Index(args, "--out")+1]
}

func TestPoppySetupApply(t *testing.T) {
	t.Run("applies the plan, writes the file last and runs doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()
		args := freshSetupArgs(t, "--yes", "--guest-api-access", "--guest-all-scopes")

		err := h.run(args...)

		require.NoError(t, err, "a doctor failure does not fail setup")
		assert.Equal(t, []string{"tenant", "api", "app", "grant"}, h.calls)

		cfg, err := config.Read(outOf(args))
		require.NoError(t, err)
		assert.Equal(t, "shop.example", cfg.Organization.Domain)

		text := h.out.String()
		assert.Contains(t, text, "Poppy setup for shop.example")
		assert.Contains(t, text, "Applying 13 changes\n")
		assert.Contains(t, text, "FAIL https://shop.example/.well-known/poppy.json could not be fetched")
		// Setup has a login, so doctor reads the tenant too. The mocks serve the
		// tenant as it was before the plan, which fails 4 of its checks, besides
		// the 2 checks of the website. No app has the browser join on and no API
		// has Poppy on yet, so the web app check is skipped and the file cannot
		// be compared with the tenant.
		assert.Contains(t, text, "FAIL client_id_metadata_document_supported is off")
		assert.NotContains(t, text, "- skipped", "the checks that did not run are not listed after setup")
		assert.NotContains(t, text, "skipped (")
		assert.Contains(t, text, "WARN cannot compare")
		assert.Contains(t, text, "6 failed")
		assert.Contains(t, text, "Checking your setup (auth0 poppy doctor)")
		assert.Contains(t, text, "written\n")
		assert.Contains(t, text, "6 checks failed. Deploy the config file with your app and run `auth0 poppy doctor shop.example` again.")
		assert.Contains(t, text, "Run auth0 poppy doctor shop.example to see every check.")
		assert.Contains(t, text, "Next\n  1. Deploy ")
		assert.Empty(t, h.messages.String(), "the result is on stdout")
	})

	t.Run("the guest grant is made only when asked for", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--skip-doctor")...))

		assert.Equal(t, []string{"tenant", "api", "app"}, h.calls)
	})

	t.Run("skip doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--skip-doctor")...))

		assert.NotContains(t, h.out.String(), "Checking your setup")
		assert.NotContains(t, h.out.String(), ".well-known")
		assert.NotContains(t, h.out.String(), "Deploy the config file")
	})

	t.Run("a tenant that is set up has nothing to change and still runs doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--guest-api-access", "--guest-all-scopes", "--out", goldenPoppyConfig(t)}

		require.NoError(t, h.run(args...))

		assert.Empty(t, h.calls)
		assert.Contains(t, h.out.String(), "Nothing to change")
		assert.NotContains(t, h.out.String(), "Applying", "nothing is applied")
		assert.Contains(t, h.out.String(), "Checking your setup")
		assert.Contains(t, h.out.String(), ".well-known/poppy.json")

		h = newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, h.run(append(args, "--skip-doctor")...))
		assert.NotContains(t, h.out.String(), "Checking your setup")
	})

	t.Run("an empty plan asks for nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--guest-api-access", "--guest-all-scopes", "--out", goldenPoppyConfig(t), "--skip-doctor"}

		require.NoError(t, h.run(args...), "no terminal and no --yes")
	})
}

func TestPoppySetupConfirmation(t *testing.T) {
	t.Run("without a terminal and without --yes nothing changes", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		args := freshSetupArgs(t)

		err := h.run(args...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "confirmation_required", reason)
		assert.Contains(t, err.Error(), "--yes")
		assert.Equal(t, poppySetupTenant, details["tenant"])
		assert.NotEmpty(t, details["changes"], "the plan travels with the error")
		assert.Contains(t, h.out.String(), "Plan: ", "the plan is printed")
		assert.NoFileExists(t, outOf(args))
	})

	t.Run("JSON output prints no plan before the error", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--json")...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "confirmation_required", reason)
		assert.NotEmpty(t, details["changes"])
		assert.Empty(t, h.out.String())
	})

	t.Run("dry run never writes, even with --yes", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		args := freshSetupArgs(t, "--dry-run", "--yes")

		require.NoError(t, h.run(args...), "the mocks have no write calls")

		assert.NoFileExists(t, outOf(args))
		assert.NotContains(t, h.messages.String(), "Doctor")
	})
}

func TestPoppySetupJSON(t *testing.T) {
	t.Run("plan, applied and doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--json")...))

		var result struct {
			Plan    setup.Plan     `json:"plan"`
			Applied []string       `json:"applied"`
			Doctor  *doctor.Report `json:"doctor"`
		}
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &result), "one object")
		require.Len(t, result.Plan.Changes, 4)
		require.Len(t, result.Applied, 4)
		assert.Equal(t, result.Plan.Changes[3].Description, result.Applied[3])
		require.NotNil(t, result.Doctor)
		assert.Equal(t, "shop.example", result.Doctor.Domain)
		assert.Equal(t, 1, result.Doctor.ExitCode())
	})

	t.Run("skip doctor leaves the doctor out", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--json-compact", "--skip-doctor")...))

		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &result))
		assert.Contains(t, result, "plan")
		assert.Contains(t, result, "applied")
		assert.NotContains(t, result, "doctor")
	})

	t.Run("an empty plan applies nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())

		require.NoError(t, h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--guest-api-access", "--guest-all-scopes", "--out", goldenPoppyConfig(t), "--skip-doctor", "--json"))

		assert.JSONEq(t, `[]`, string(jsonField(t, h.out.Bytes(), "applied")))
	})
}

func jsonField(t *testing.T, data []byte, name string) json.RawMessage {
	t.Helper()

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fields))

	return fields[name]
}

func TestPoppySetupFailures(t *testing.T) {
	forbidden := func(body string) error { return core.NewAPIError(http.StatusForbidden, nil, errors.New(body)) }

	tests := []struct {
		name        string
		fail        string
		err         error
		wantCalls   []string
		wantPartial bool
		wantMessage string
		wantReason  string
	}{
		{name: "the first change fails", fail: "tenant", err: errors.New("boom"), wantCalls: []string{"tenant"}, wantMessage: "failed to update the tenant settings: boom"},
		{name: "the API fails", fail: "api", err: errors.New("boom"), wantCalls: []string{"tenant", "api"}, wantPartial: true, wantMessage: "failed to update the API"},
		{
			name: "a 400 is printed as the platform sends it", fail: "app", err: core.NewAPIError(http.StatusBadRequest, nil, errors.New("invalid_personal_agent_browser_join_client")),
			wantCalls: []string{"tenant", "api", "app"}, wantPartial: true, wantMessage: "invalid_personal_agent_browser_join_client",
		},
		{
			name: "a 403 for a missing scope", fail: "api", err: forbidden(`{"errorCode":"insufficient_scope","message":"Insufficient scope, expected any of: update:resource_servers"}`),
			wantCalls: []string{"tenant", "api"}, wantPartial: true, wantMessage: "auth0 login --scopes update:resource_servers",
		},
		{
			name: "any other 403", fail: "grant", err: forbidden(`{"errorCode":"poppy_beta_off"}`),
			wantCalls: []string{"tenant", "api", "app", "grant"}, wantPartial: true, wantMessage: "the tenant may lack Agent as Principal or the Poppy beta",
		},
		{
			name: "a 403 on the first change", fail: "tenant", err: forbidden(`{"errorCode":"poppy_beta_off"}`),
			wantCalls: []string{"tenant"}, wantMessage: "the tenant may lack Agent as Principal or the Poppy beta",
		},
	}

	for _, test := range tests {
		for _, mode := range []struct {
			name  string
			flags []string
		}{{name: "human"}, {name: "JSON", flags: []string{"--json"}}} {
			t.Run(test.name+" ("+mode.name+")", func(t *testing.T) {
				h := newPoppySetupHarness(t, freshPoppyTenant())
				h.allowWrites()
				h.fail[test.fail] = test.err
				args := freshSetupArgs(t, append([]string{"--yes", "--guest-api-access", "--guest-all-scopes"}, mode.flags...)...)

				err := h.run(args...)

				require.Error(t, err)
				assert.Equal(t, test.wantCalls, h.calls, "stops at the first failure")
				assert.Contains(t, err.Error(), test.wantMessage)
				assert.NoFileExists(t, outOf(args), "the file is written last")
				assert.NotContains(t, h.out.String(), "Checking your setup", "doctor runs only after a full apply")

				var partial *setup.PartialError
				isPartial := errors.As(err, &partial)

				switch {
				case !test.wantPartial:
					assert.False(t, isPartial)
				case len(mode.flags) > 0:
					require.True(t, isPartial, "the JSON error keeps the whole account")
					assert.Contains(t, err.Error(), "stopped partway")
					assert.Contains(t, err.Error(), "Running the command again continues")
					assert.Equal(t, "partial", decodeDetails(t, err)["code"])
				default:
					// The step list says what was applied and how to continue, so the error is
					// only the cause and the account is not printed a second time.
					assert.False(t, isPartial, "a person gets the cause only")
					assert.NotContains(t, err.Error(), "Running the command again continues")
					assert.NotContains(t, err.Error(), "stopped partway")
					assert.Equal(t, 1, strings.Count(h.out.String(), "Running the command again continues from here."))
					assert.Contains(t, h.out.String(), "To rerun:")
					assert.Contains(t, h.out.String(), "not run")
				}
			})
		}
	}
}

func TestPoppyJSONLeavesNoText(t *testing.T) {
	for _, flag := range []string{"--json", "--json-compact"} {
		tests := map[string]struct {
			tenant func() poppyTenantState
			args   func(t *testing.T) []string
		}{
			"setup":         {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag, "--yes", "--skip-doctor") }},
			"setup dry run": {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag, "--dry-run") }},
			"setup refused": {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag) }},
			"config": {configuredPoppyTenant, func(t *testing.T) []string {
				return []string{"config", flag, "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
					"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", filepath.Join(t.TempDir(), "auth0-poppy.json")}
			}},
		}

		for name, test := range tests {
			t.Run(flag+" "+name, func(t *testing.T) {
				h := newPoppySetupHarness(t, test.tenant())
				h.allowWrites()

				_ = h.run(test.args(t)...)

				assert.Empty(t, h.messages.String(), "nothing on stderr")

				if h.out.Len() > 0 {
					assert.True(t, json.Valid(h.out.Bytes()), "nothing but the JSON on stdout:\n%s", h.out.String())
				}
			})
		}
	}
}

func TestPoppySetupFailureText(t *testing.T) {
	t.Run("the steps made, the one that failed and the ones not run", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()
		h.fail["app"] = errors.New("boom")

		err := h.run(freshSetupArgs(t, "--yes", "--guest-api-access", "--guest-all-scopes")...)

		require.Error(t, err)

		out := h.out.String()
		assert.Contains(t, out, "Applying 13 changes\n")
		assert.Contains(t, out, "  ok Tenant settings")
		assert.Contains(t, out, "5 changes")
		assert.Contains(t, out, "  x  App Shop Web")
		assert.Contains(t, out, "failed\n")
		assert.Contains(t, out, "boom")
		assert.Contains(t, out, "  -  Guest grant "+poppyTestAPI)
		assert.Contains(t, out, "not run")
		assert.Contains(t, out, "Running the command again continues from here.\nTo rerun:  auth0 poppy setup --company-domain shop.example --api "+poppyTestAPI)
		assert.Contains(t, out, "--guest-all-scopes --yes")
		assert.NotContains(t, out, "--dry-run")
		assert.Empty(t, h.messages.String())
	})

	t.Run("the output flags print no block", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			h := newPoppySetupHarness(t, freshPoppyTenant())
			h.allowWrites()
			h.fail["app"] = errors.New("boom")

			err := h.run(freshSetupArgs(t, "--yes", flag)...)

			require.Error(t, err, flag)
			assert.Empty(t, h.out.String(), flag)
			assert.Empty(t, h.messages.String(), flag)
		}
	})
}

func decodeDetails(t *testing.T, err error) map[string]any {
	t.Helper()

	var details map[string]any
	require.NoError(t, json.Unmarshal(errorDetails(err), &details))

	return details
}

func TestPoppySetupPreflight(t *testing.T) {
	keyring.MockInit()

	home := t.TempDir()
	t.Setenv("HOME", home)

	// The login holds every read scope but no scope to write.
	payload, err := json.Marshal(map[string]string{"scope": strings.Join(setup.ReadScopes, " ")})
	require.NoError(t, err)
	token := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	saved, err := json.Marshal(map[string]any{
		"default_tenant": poppySetupTenant,
		"tenants":        map[string]any{poppySetupTenant: map[string]any{"domain": poppySetupTenant, "access_token": token}},
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "auth0"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "auth0", "config.json"), saved, 0o600))

	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()
	args := freshSetupArgs(t, "--yes")

	err = h.run(args...)

	require.Error(t, err)
	assert.Equal(t, "missing_scopes", errorReason(err))
	assert.Contains(t, err.Error(), "auth0 login --scopes ")
	assert.Contains(t, err.Error(), "update:tenant_settings")
	assert.Empty(t, h.calls, "nothing is changed")
	assert.NoFileExists(t, outOf(args))

	h = newPoppySetupHarness(t, freshPoppyTenant())
	require.NoError(t, h.run(freshSetupArgs(t, "--dry-run")...), "a dry run needs only the read scopes")
}

func TestCheckPoppyScopesModes(t *testing.T) {
	keyring.MockInit()

	home := t.TempDir()
	t.Setenv("HOME", home)

	// The saved login holds no scope the commands need.
	payload, err := json.Marshal(map[string]string{"scope": "openid"})
	require.NoError(t, err)
	token := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	saved, err := json.Marshal(map[string]any{
		"default_tenant": poppySetupTenant,
		"tenants":        map[string]any{poppySetupTenant: map[string]any{"domain": poppySetupTenant, "access_token": token}},
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "auth0"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "auth0", "config.json"), saved, 0o600))

	t.Run("a saved login is checked", func(t *testing.T) {
		t.Setenv(authModeEnvVar, "")

		err := (&cli{tenant: poppySetupTenant}).checkPoppyScopes([]string{"read:clients"})

		require.Error(t, err)
		assert.Equal(t, "missing_scopes", errorReason(err))
	})

	t.Run("the saved login is not consulted in env auth mode", func(t *testing.T) {
		t.Setenv(authModeEnvVar, authModeEnv)

		assert.NoError(t, (&cli{tenant: poppySetupTenant}).checkPoppyScopes([]string{"read:clients"}))
	})
}

func TestPoppySetupGuestScopes(t *testing.T) {
	t.Run("the scopes are required without a terminal and nothing changes", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		args := freshSetupArgs(t, "--yes", "--guest-api-access")

		err := h.run(args...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "scopes_required", reason)
		assert.Contains(t, err.Error(), "pass --guest-scopes or --guest-all-scopes")
		assert.Contains(t, err.Error(), poppyTestAPI+": orders:read, checkout")
		assert.Contains(t, err.Error(), "\n  FIX  ")
		assert.NotEmpty(t, details["apis"], "the offer travels with the error")
		assert.Empty(t, h.calls)
		assert.NoFileExists(t, outOf(args))
	})

	t.Run("JSON output fails the same way", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--yes", "--json", "--guest-api-access")...)

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "scopes_required", reason)
	})

	t.Run("a missing description is reported before the scopes", func(t *testing.T) {
		state := freshPoppyTenant()
		state.server.Scopes[1].Description = nil
		h := newPoppySetupHarness(t, state)

		err := h.run(freshSetupArgs(t, "--yes", "--guest-api-access")...)

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "scope_description_required", reason)
	})

	t.Run("named scopes are the only ones in the plan and the grant", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--dry-run", "--guest-api-access", "--guest-scopes", "orders:read")...))
		assert.Empty(t, h.calls)
		assert.Contains(t, h.out.String(), "scopes: orders:read\n")

		h = newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--skip-doctor", "--guest-api-access", "--guest-scopes", "orders:read")...))
		assert.Equal(t, []string{"tenant", "api", "app", "grant"}, h.calls)
		assert.Equal(t, []string{"orders:read"}, h.grant.Scope)
	})

	t.Run("the plan shows exactly the named scopes", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(freshSetupArgs(t, "--dry-run", "--json", "--guest-api-access", "--guest-scopes", "orders:read")...))

		var plan setup.Plan
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &plan))
		var scope string
		for _, change := range plan.Changes {
			if change.Kind == setup.KindGrant {
				for _, field := range change.Fields {
					if field.Name == "scope" {
						scope = field.Value
					}
				}
			}
		}
		assert.Equal(t, "orders:read", scope)
	})

	t.Run("every scope", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--yes", "--skip-doctor", "--guest-api-access", "--guest-all-scopes")...))

		assert.Equal(t, []string{"orders:read", "checkout", setup.BrowserSignInScope}, h.grant.Scope)
	})

	t.Run("a scope the API lacks", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--yes", "--guest-api-access", "--guest-scopes", "nope")...)

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "unknown_scope", reason)
		assert.Contains(t, err.Error(), `scope "nope" is not defined on `+poppyTestAPI)
	})

	t.Run("the scope flags need the guest grant", func(t *testing.T) {
		for _, flag := range [][]string{{"--guest-scopes", "orders:read"}, {"--guest-all-scopes"}} {
			h := newPoppySetupHarness(t, freshPoppyTenant())

			err := h.run(freshSetupArgs(t, append([]string{"--yes"}, flag...)...)...)

			reason, _ := decodeCLIError(t, err)
			assert.Equal(t, "invalid_flag", reason, flag[0])
			assert.Contains(t, err.Error(), "\n  FIX  Add --guest-api-access.", flag[0])
			assert.Empty(t, h.calls)
		}
	})

	t.Run("the two scope flags exclude each other", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--guest-api-access", "--guest-scopes", "orders:read", "--guest-all-scopes")...)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "none of the others can be")
	})

	t.Run("a rerun on a configured tenant needs no scope flag and changes nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--guest-api-access", "--out", goldenPoppyConfig(t), "--skip-doctor"}

		require.NoError(t, h.run(args...), "no write call is set on the mocks")

		assert.Contains(t, h.out.String(), "Nothing to change")
	})
}

// guestPromptScript answers the guest questions in order and records what was
// asked.
type guestPromptScript struct {
	allow  bool
	all    bool
	picks  map[string][]string
	failOn string

	asked []string
	notes []string
}

func (s *guestPromptScript) prompts() poppyGuestPrompts {
	return poppyGuestPrompts{
		allow: func() (bool, error) {
			s.asked = append(s.asked, "allow")

			return s.allow, nil
		},
		all: func() (bool, error) {
			s.asked = append(s.asked, "all")
			if s.failOn == "all" {
				return false, errors.New("interrupted")
			}

			return s.all, nil
		},
		pick: func(audience string, _ []setup.GuestScope) ([]string, error) {
			s.asked = append(s.asked, "pick "+audience)

			return s.picks[audience], nil
		},
		note: func(format string, args ...any) { s.notes = append(s.notes, fmt.Sprintf(format, args...)) },
	}
}

func guestTestState(guest map[string]bool) *setup.State {
	scopes := []setup.Scope{
		{Name: "orders:read", Description: "See your orders and their status. Read only."},
		{Name: "checkout", Description: "Place orders and pay with your saved card."},
	}

	return &setup.State{
		APIs: []setup.API{
			{Identifier: poppyTestAPI, Scopes: scopes},
			{Identifier: "https://other.example", Scopes: scopes[:1]},
		},
		HasGuestGrant: guest,
	}
}

func TestChoosePoppyGuestScopes(t *testing.T) {
	const other = "https://other.example"

	tests := []struct {
		name        string
		interactive bool
		in          setup.Inputs
		guest       map[string]bool
		script      guestPromptScript
		wantAsked   []string
		wantAccess  bool
		wantAll     bool
		wantScopes  map[string][]string
		wantReason  string
		wantNotes   int
	}{
		{name: "without a terminal and without the guest grant nothing is asked", in: setup.Inputs{}, wantAsked: nil},
		{
			name:        "in a terminal the question about guests is asked first, default no",
			interactive: true,
			wantAsked:   []string{"allow"},
		},
		{
			name:        "no guests, no more questions",
			interactive: true,
			script:      guestPromptScript{allow: false, all: true},
			wantAsked:   []string{"allow"},
		},
		{
			name:        "a guest grant on only one API still asks the question",
			interactive: true,
			guest:       map[string]bool{other: true},
			wantAsked:   []string{"allow"},
		},
		{
			name:        "a guest grant on every API means the question is not asked",
			interactive: true,
			guest:       map[string]bool{poppyTestAPI: true, other: true},
			wantAsked:   nil,
		},
		{
			name:        "yes, then all scopes",
			interactive: true,
			script:      guestPromptScript{allow: true, all: true},
			wantAsked:   []string{"allow", "all"},
			wantAccess:  true,
			wantAll:     true,
		},
		{
			name:        "yes, then a selection on each API",
			interactive: true,
			script:      guestPromptScript{allow: true, picks: map[string][]string{poppyTestAPI: {"orders:read"}, other: {"orders:read"}}},
			wantAsked:   []string{"allow", "all", "pick " + poppyTestAPI, "pick " + other},
			wantAccess:  true,
			wantScopes:  map[string][]string{poppyTestAPI: {"orders:read"}, other: {"orders:read"}},
		},
		{
			name:        "the guest grant flag skips the first question",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true},
			script:      guestPromptScript{all: true},
			wantAsked:   []string{"all"},
			wantAccess:  true,
			wantAll:     true,
		},
		{
			name:        "an API with nothing chosen gets no guest grant",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true},
			script:      guestPromptScript{picks: map[string][]string{poppyTestAPI: {"checkout"}}},
			wantAsked:   []string{"all", "pick " + poppyTestAPI, "pick " + other},
			wantAccess:  true,
			wantScopes:  map[string][]string{poppyTestAPI: {"checkout"}},
			wantNotes:   1,
		},
		{
			name:        "nothing chosen anywhere",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true},
			wantAsked:   []string{"all", "pick " + poppyTestAPI, "pick " + other},
			wantAccess:  true,
			wantScopes:  map[string][]string{},
			wantNotes:   1,
		},
		{
			name:        "an API that has a guest grant is not listed",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true},
			guest:       map[string]bool{other: true},
			script:      guestPromptScript{picks: map[string][]string{poppyTestAPI: {"orders:read"}}},
			wantAsked:   []string{"all", "pick " + poppyTestAPI},
			wantAccess:  true,
			wantScopes:  map[string][]string{poppyTestAPI: {"orders:read"}},
		},
		{
			name:        "named scopes skip both scope questions",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true, GuestScopeNames: []string{"orders:read"}},
			wantAccess:  true,
			wantScopes:  map[string][]string{poppyTestAPI: {"orders:read"}, other: {"orders:read"}},
		},
		{
			name:        "every scope skips the scope questions",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true, GuestAllScopes: true},
			wantAccess:  true,
			wantAll:     true,
		},
		{
			name:        "an interrupted question stops",
			interactive: true,
			in:          setup.Inputs{GuestAPIAccess: true},
			script:      guestPromptScript{failOn: "all"},
			wantAsked:   []string{"all"},
			wantReason:  "interrupted",
			wantAccess:  true,
		},
		{
			name:       "without a terminal the scopes are required",
			in:         setup.Inputs{GuestAPIAccess: true},
			wantReason: "scopes_required",
			wantAccess: true,
		},
		{
			name:       "without a terminal a scope that one API lacks is refused",
			in:         setup.Inputs{GuestAPIAccess: true, GuestScopeNames: []string{"checkout"}},
			wantReason: "unknown_scope",
			wantAccess: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := test.in

			err := choosePoppyGuestScopes(test.interactive, test.script.prompts(), guestTestState(test.guest), &in)

			assert.Equal(t, test.wantAsked, test.script.asked)
			assert.Len(t, test.script.notes, test.wantNotes)

			if test.wantReason != "" {
				require.Error(t, err)
				if test.wantReason == "interrupted" {
					assert.ErrorContains(t, err, "interrupted")
				} else {
					assert.Equal(t, test.wantReason, errorReason(err))
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantAccess, in.GuestAPIAccess)
			assert.Equal(t, test.wantAll, in.GuestAllScopes)
			assert.Equal(t, test.wantScopes, in.GuestScopes)
		})
	}
}

func TestPoppySetupScopeDescriptionsWithoutATerminal(t *testing.T) {
	undescribed := func() poppyTenantState {
		state := freshPoppyTenant()
		state.server.Scopes[1].Description = nil

		return state
	}

	tests := []struct {
		name string
		args func(t *testing.T) []string
	}{
		{name: "no terminal", args: func(t *testing.T) []string { return freshSetupArgs(t) }},
		{name: "JSON output", args: func(t *testing.T) []string { return freshSetupArgs(t, "--json") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, undescribed())

			err := h.run(test.args(t)...)

			reason, _ := decodeCLIError(t, err)
			assert.Equal(t, "scope_description_required", reason)
			assert.Contains(t, err.Error(), "These have none: checkout ("+poppyTestAPI+").")
			assert.Contains(t, err.Error(), "\n  FIX  Pass --scope 'checkout=<what this lets an agent do>'.")
			assert.Empty(t, h.calls)
		})
	}

	t.Run("--scope answers it", func(t *testing.T) {
		h := newPoppySetupHarness(t, undescribed())

		require.NoError(t, h.run(freshSetupArgs(t, "--dry-run", "--scope", "checkout=Place orders")...))
		assert.Contains(t, h.out.String(), `~  scope checkout`)
		assert.Contains(t, h.out.String(), `none -> "Place orders"`)
		assert.Contains(t, h.out.String(), `+  scope poppy:browser-sign-in`)
	})
}

func TestIsPoppyScopeDescriptionRequired(t *testing.T) {
	assert.True(t, isPoppyScopeDescriptionRequired(&setup.InputError{Reason: setup.ReasonScopeDescriptionRequired}))
	assert.True(t, isPoppyScopeDescriptionRequired(fmt.Errorf("wrapped: %w", &setup.InputError{Reason: setup.ReasonScopeDescriptionRequired})))
	assert.False(t, isPoppyScopeDescriptionRequired(&setup.InputError{Reason: "app_not_first_party"}))
	assert.False(t, isPoppyScopeDescriptionRequired(errors.New("other")))
	assert.False(t, isPoppyScopeDescriptionRequired(nil))
}

func TestDescribePoppyScopes(t *testing.T) {
	state := configuredPoppyStateForScopes()

	const (
		ordersTitle   = "Description of orders:read"
		checkoutTitle = "Description of checkout"
	)

	tests := []struct {
		name        string
		answers     []string
		askErr      error
		wantLabels  []string
		wantScopes  map[string]string
		wantInvalid int
	}{
		{
			name:       "each scope is asked about and stored as if --scope had been passed",
			answers:    []string{"See orders", "Place orders"},
			wantLabels: []string{"Text", "Text"},
			wantScopes: map[string]string{"orders:read": "See orders", "checkout": "Place orders"},
		},
		{
			name:        "an empty answer asks again",
			answers:     []string{"", "  ", "See orders", "Place orders"},
			wantLabels:  []string{"Text", "Text", "Text", "Text"},
			wantScopes:  map[string]string{"orders:read": "See orders", "checkout": "Place orders"},
			wantInvalid: 2,
		},
		{
			name:       "an interrupted prompt stops",
			askErr:     errors.New("interrupt"),
			wantLabels: []string{"Text"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := setup.Inputs{}

			var labels []string
			next := 0

			screen, out := newTestPoppyScreen(t, poppyAsker{
				text: func(_ *poppyScreen, label, _ string) (string, error) {
					labels = append(labels, label)
					if test.askErr != nil {
						return "", test.askErr
					}

					answer := test.answers[next]
					next++

					return answer, nil
				},
			}, false)

			err := describePoppyScopes(screen, setup.ScopeQuestions(state, in), &in)

			assert.Equal(t, test.wantLabels, labels)
			assert.Contains(t, out.String(), ordersTitle)
			assert.Equal(t, test.wantInvalid, strings.Count(out.String(), "A value is required."))

			if test.askErr != nil {
				assert.EqualError(t, err, test.askErr.Error())

				return
			}

			require.NoError(t, err)
			assert.Contains(t, out.String(), checkoutTitle)
			assert.Equal(t, test.wantScopes, in.Scopes)
			assert.Empty(t, setup.ScopeQuestions(state, in), "nothing is left to ask")
		})
	}
}

// configuredPoppyStateForScopes is a state whose two scopes have no description.
func configuredPoppyStateForScopes() *setup.State {
	return &setup.State{
		APIs: []setup.API{{
			ID:         "rs_shop",
			Identifier: poppyTestAPI,
			Scopes:     []setup.Scope{{Name: "orders:read"}, {Name: "checkout"}},
		}},
	}
}

// runPromptMode runs a setup command of the harness in a mode and returns what it did.
func (h *poppySetupHarness) runPromptMode(t *testing.T, mode poppyPromptMode, args []string) poppyPromptRun {
	t.Helper()

	asked := installPoppyAsker(t, mode)
	h.agentMode = mode.agentMode

	err := h.run(append(args, mode.flags...)...)

	return poppyPromptRun{asked: *asked, out: h.out.String(), errOut: h.messages.String(), err: err}
}

// checkPoppySetupPromptRows runs setup with the extra flags against the tenant
// in each mode and compares it with the row of the mode.
func checkPoppySetupPromptRows(t *testing.T, tenant poppyTenantState, extra []string, tests []poppyPromptRow) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, tenant)
			h.allowWrites()

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, freshSetupArgs(t, extra...)))
		})
	}
}

func TestPoppySetupPromptRule(t *testing.T) {
	const plan = "Poppy setup for shop.example"

	asks := []string{"hints", "guest access", "guest scopes", "confirm"}

	checkPoppySetupPromptRows(t, freshPoppyTenant(), []string{"--skip-doctor"}, []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: plan, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: plan, reason: "confirmation_required", errText: "--yes"},
		{name: "piped", mode: modePiped, text: plan, reason: "confirmation_required", errText: "--yes"},
		{name: "piped with --json", mode: modePipedJSON, reason: "confirmation_required", errText: "--yes"},
		{name: "agent mode", mode: modeAgent, reason: "confirmation_required", errText: "--yes"},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: plan, json: true},
	})
}

func TestPoppySetupPromptRuleScopeDescription(t *testing.T) {
	const (
		plan    = "Poppy setup for shop.example"
		missing = "FIX  Pass --scope 'checkout=<what this lets an agent do>'."
	)

	asks := []string{"text Text", "hints", "confirm"}

	tenant := freshPoppyTenant()
	tenant.server.Scopes[1].Description = nil

	checkPoppySetupPromptRows(t, tenant, []string{"--skip-doctor", "--guest-api-access", "--guest-all-scopes"}, []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: plan, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "scope_description_required", errText: missing},
		{name: "piped", mode: modePiped, reason: "scope_description_required", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "scope_description_required", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "scope_description_required", errText: missing},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: plan, json: true},
	})
}

func TestPoppySetupPromptRuleGuestScopes(t *testing.T) {
	const (
		plan    = "Poppy setup for shop.example"
		missing = "pass --guest-scopes or --guest-all-scopes"
	)

	asks := []string{"hints", "guest scopes"}

	checkPoppySetupPromptRows(t, freshPoppyTenant(), []string{"--skip-doctor", "--yes", "--guest-api-access"}, []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: plan, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "scopes_required", errText: missing},
		{name: "piped", mode: modePiped, reason: "scopes_required", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "scopes_required", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "scopes_required", errText: missing},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: plan, json: true},
	})
}

func TestPoppyConfigPromptRule(t *testing.T) {
	// A terminal is asked about the hints of the scopes, and the test terminal
	// cannot take the question back, so it stays on the screen.
	const question = "Hints for your scopes"

	asks := []string{"hints"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "written"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: question, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "written"},
		{name: "piped", mode: modePiped, text: "written"},
		{name: "piped with --json", mode: modePipedJSON, json: true},
		{name: "agent mode", mode: modeAgent, json: true},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: question, json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, configuredPoppyTenant())
			args := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
				"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", filepath.Join(t.TempDir(), "auth0-poppy.json")}

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, args))
		})
	}
}

func TestPoppyConfigPromptRuleCompanyDomain(t *testing.T) {
	const missing = "--company-domain is required"

	// The question a terminal gets here is a survey prompt that no test can answer,
	// so the modes that ask are left out.
	tests := []poppyPromptRow{
		{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "missing_flag", errText: missing},
		{name: "piped", mode: modePiped, reason: "missing_flag", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "missing_flag", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "missing_flag", errText: missing},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tenant := configuredPoppyTenant()
			tenant.settings.PersonalAgentProtocolCompanyDomains = []string{"https://shop.example", "https://shop.test"}

			h := newPoppySetupHarness(t, tenant)
			args := []string{"config", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "auth0-poppy.json")}

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, args))
		})
	}
}

// poppyScript answers the prompts of a test in order and keeps what they asked.
// An answer is a bool for a question of yes or no, an int for a list, an error
// for Ctrl-C, and nil for Enter, which takes the default.
type poppyScript struct {
	answers []any
	asked   []string
}

func (s *poppyScript) next(t *testing.T, message string) any {
	t.Helper()

	s.asked = append(s.asked, message)
	require.NotEmpty(t, s.answers, "no answer was scripted for %q", message)

	answer := s.answers[0]
	s.answers = s.answers[1:]

	return answer
}

func (s *poppyScript) asker(t *testing.T) poppyAsker {
	return poppyAsker{
		confirm: func(_ *poppyScreen, message string, def bool) (bool, error) {
			switch answer := s.next(t, message).(type) {
			case nil:
				return def, nil
			case bool:
				return answer, nil
			case error:
				return false, answer
			default:
				require.Failf(t, "wrong answer", "%T for a question of yes or no", answer)

				return false, nil
			}
		},
		choose: func(_ *poppyScreen, message string, _ []string, def int) (int, error) {
			switch answer := s.next(t, message).(type) {
			case nil:
				return def, nil
			case int:
				return answer, nil
			case error:
				return 0, answer
			default:
				require.Failf(t, "wrong answer", "%T for a list", answer)

				return 0, nil
			}
		},
	}
}

var hintScopes = []setup.HintScope{
	{Name: "orders:read", Description: "See your orders and their status.", APIs: []string{"https://api.shop.example"}},
	{Name: "checkout", Description: "Place orders and pay with your saved card.", APIs: []string{"https://api.shop.example"}},
	{Name: "refund", Description: "Refund an order you placed.", APIs: []string{"https://api.shop.example", "https://other.example"}},
}

const (
	introQuestion  = "Set them now?"
	sameQuestion   = "Use the same answers for the remaining 2 scopes?"
	changeQuestion = poppyChangesQuestion
	personalData   = poppyPersonalDataQuestion
)

func TestAskPoppyHints(t *testing.T) {
	const (
		none         = 0
		reversible   = 1
		irreversible = 2
	)

	cautious := config.Hints{Changes: "irreversible", PersonalData: true}

	tests := []struct {
		name         string
		scopes       []setup.HintScope
		answers      []any
		wantAsked    []string
		wantHints    map[string]config.Hints
		wantUnhinted int
		wantErr      error
	}{
		{
			name:    "Enter on every question is the cautious answer for every scope",
			scopes:  hintScopes,
			answers: make([]any, 8),
			wantAsked: []string{
				introQuestion,
				changeQuestion, personalData, sameQuestion,
				changeQuestion, personalData,
				changeQuestion, personalData,
			},
			wantHints: map[string]config.Hints{"orders:read": cautious, "checkout": cautious, "refund": cautious},
		},
		{
			name:    "mixed answers",
			scopes:  hintScopes,
			answers: []any{nil, none, false, false, reversible, true, nil, false},
			wantAsked: []string{
				introQuestion,
				changeQuestion, personalData, sameQuestion,
				changeQuestion, personalData,
				changeQuestion, personalData,
			},
			wantHints: map[string]config.Hints{
				"orders:read": {Changes: "none"},
				"checkout":    {Changes: "reversible", PersonalData: true},
				"refund":      {Changes: "irreversible"},
			},
		},
		{
			name:      "the same answers for the remaining scopes",
			scopes:    hintScopes,
			answers:   []any{nil, none, false, true},
			wantAsked: []string{introQuestion, changeQuestion, personalData, sameQuestion},
			wantHints: map[string]config.Hints{"orders:read": {Changes: "none"}, "checkout": {Changes: "none"}, "refund": {Changes: "none"}},
		},
		{
			name:         "skipping all leaves the hints unset",
			scopes:       hintScopes,
			answers:      []any{false},
			wantAsked:    []string{introQuestion},
			wantUnhinted: 3,
		},
		{
			name:      "one scope has its own wording and no shortcut",
			scopes:    hintScopes[:1],
			answers:   []any{nil, irreversible, nil},
			wantAsked: []string{"Set it now?", changeQuestion, personalData},
			wantHints: map[string]config.Hints{"orders:read": cautious},
		},
		{
			name:      "two scopes offer the shortcut for the one that remains",
			scopes:    hintScopes[:2],
			answers:   []any{nil, none, false, nil, nil, nil},
			wantAsked: []string{introQuestion, changeQuestion, personalData, "Use the same answers for the remaining 1 scope?", changeQuestion, personalData},
			wantHints: map[string]config.Hints{"orders:read": {Changes: "none"}, "checkout": cautious},
		},
		{
			name:         "no scopes, no questions",
			scopes:       nil,
			wantUnhinted: 0,
		},
		{
			name:      "Ctrl-C at the first question",
			scopes:    hintScopes,
			answers:   []any{terminal.InterruptErr},
			wantAsked: []string{introQuestion},
			wantErr:   terminal.InterruptErr,
		},
		{
			name:      "Ctrl-C at the second question of a scope",
			scopes:    hintScopes,
			answers:   []any{nil, none, terminal.InterruptErr},
			wantAsked: []string{introQuestion, changeQuestion, personalData},
			wantErr:   terminal.InterruptErr,
		},
		{
			name:      "Ctrl-C at the shortcut",
			scopes:    hintScopes,
			answers:   []any{nil, none, false, terminal.InterruptErr},
			wantAsked: []string{introQuestion, changeQuestion, personalData, sameQuestion},
			wantErr:   terminal.InterruptErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			script := &poppyScript{answers: test.answers}
			screen, out := newTestPoppyScreen(t, script.asker(t), false)
			screen.ForDomain("shop.example")

			in := setup.Inputs{}
			unhinted, err := askPoppyHints(screen, test.scopes, &in)

			assert.Equal(t, test.wantAsked, script.asked)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.True(t, strings.HasSuffix(out.String(), display.PoppyCancelled+"\n"), "the last line says nothing was changed:\n%s", out.String())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantUnhinted, unhinted)
			assert.Equal(t, test.wantHints, in.Hints)
			assert.NotContains(t, out.String(), display.PoppyCancelled)
			assert.Empty(t, script.answers, "every scripted answer was used")
		})
	}
}

func TestAskPoppyHintsScreen(t *testing.T) {
	script := &poppyScript{answers: []any{nil, 0, false, false, nil, nil}}
	screen, out := newTestPoppyScreen(t, script.asker(t), false)
	screen.ForDomain("shop.example")

	_, err := askPoppyHints(screen, hintScopes[:2], &setup.Inputs{})
	require.NoError(t, err)

	want := ansi.Bold("Poppy setup for shop.example") + strings.Repeat(" ", 30) + "tenant mock.auth0.test\n" +
		"\n" +
		"Hints for your scopes\n" +
		"2 scopes have no hints yet. Agents show them to the user before asking for\n" +
		"approval.\n" +
		"\n" +
		"Hints for orders:read on https://api.shop.example  (1 of 2)\n" +
		"See your orders and their status.\n" +
		"  ✓ Does it change anything?  none\n" +
		"  ✓ Does it show or change personal data?  No\n" +
		"\n" +
		"Hints for checkout on https://api.shop.example  (2 of 2)\n" +
		"Place orders and pay with your saved card.\n" +
		"  ✓ Does it change anything?  irreversible\n" +
		"  ✓ Does it show or change personal data?  Yes\n"

	assert.Equal(t, want, stripEscapes(out.String()), "the title of the run is printed once, above the first question")
}

// stripEscapes removes the escape sequences of the colors and of the cursor.
var escapes = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\r`)

func stripEscapes(s string) string { return escapes.ReplaceAllString(s, "") }

func TestAskPoppyHintsErase(t *testing.T) {
	t.Run("every block is taken back as soon as it is answered", func(t *testing.T) {
		script := &poppyScript{answers: []any{nil, nil, nil, false, nil, nil}}
		screen, out := newTestPoppyScreen(t, script.asker(t), true)
		screen.ForDomain("shop.example")

		_, err := askPoppyHints(screen, hintScopes[:2], &setup.Inputs{})
		require.NoError(t, err)

		// The intro, and the block of each of the two scopes.
		erases := regexp.MustCompile(`\r\x1b\[(\d+)A\x1b\[J`).FindAllStringSubmatch(out.String(), -1)
		require.Len(t, erases, 3)

		// The intro takes a blank line, the title, and the reason on two rows. A scope
		// takes the same, with a reason of one row, and the two answers that stay in
		// view. The rows of the prompts themselves are counted by the prompts.
		assert.Equal(t, "4", erases[0][1])
		assert.Equal(t, "5", erases[1][1])
		assert.Equal(t, "5", erases[2][1])

		assert.Equal(t, 1, strings.Count(out.String(), "Poppy setup for shop.example"), "the title of the run is not taken back")
		assert.NotContains(t, out.String(), display.PoppyCancelled)
	})

	t.Run("Ctrl-C takes the question back and says nothing was changed", func(t *testing.T) {
		script := &poppyScript{answers: []any{nil, terminal.InterruptErr}}
		screen, out := newTestPoppyScreen(t, script.asker(t), true)
		screen.ForDomain("shop.example")

		_, err := askPoppyHints(screen, hintScopes, &setup.Inputs{})

		require.ErrorIs(t, err, terminal.InterruptErr)

		got := out.String()
		assert.Regexp(t, `\r\x1b\[\d+A\x1b\[J`+display.PoppyCancelled+`\n$`, got, "the question is erased, then one line is printed")
		assert.Equal(t, 1, strings.Count(got, display.PoppyCancelled))
		assert.Equal(t, 3, strings.Count(got, "\x1b[J"), "the intro, the question, and the title of the run are taken back")
	})

	t.Run("a terminal that cannot take lines back leaves them", func(t *testing.T) {
		script := &poppyScript{answers: make([]any, 4)}
		screen, out := newTestPoppyScreen(t, script.asker(t), false)

		_, err := askPoppyHints(screen, hintScopes[:1], &setup.Inputs{})
		require.NoError(t, err)

		assert.NotContains(t, out.String(), "\x1b[")
		assert.Contains(t, out.String(), "Hints for orders:read on https://api.shop.example  (1 of 1)")
	})
}

// hintFlow runs setup in a terminal against a fresh tenant with the answers
// scripted for the questions about hints, and the guest grant answered by the
// flags. The questions that are not about hints are not expected.
func runHintFlow(t *testing.T, answers []any, extra ...string) (poppyPromptRun, string) {
	t.Helper()

	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()

	args := freshSetupArgs(t, append([]string{"--skip-doctor", "--guest-api-access", "--guest-all-scopes"}, extra...)...)

	script := &poppyScript{answers: answers}
	run := h.runHintMode(t, script, args)
	run.asked = script.asked

	return run, outOf(args)
}

// runHintMode runs setup in a terminal that answers the hint questions from the
// script and every other question with yes.
func (h *poppySetupHarness) runHintMode(t *testing.T, script *poppyScript, args []string) poppyPromptRun {
	t.Helper()

	installPoppyAsker(t, modeTerminal)

	scripted := script.asker(t)
	previous := poppyAsk.confirm
	poppyAsk.confirm = func(s *poppyScreen, message string, def bool) (bool, error) {
		if strings.HasPrefix(message, "Set ") || strings.HasPrefix(message, "Use the same") || strings.HasPrefix(message, "Does it") {
			return scripted.confirm(s, message, def)
		}

		return previous(s, message, def)
	}
	poppyAsk.choose = scripted.choose
	poppyAsk.interrupt = func(err error) error { return err }

	err := h.run(args...)

	return poppyPromptRun{out: h.out.String(), errOut: h.messages.String(), err: err}
}

func TestPoppySetupHintAnswersEqualTheFlags(t *testing.T) {
	// The cautious answer for orders:read is changed to none and no personal data,
	// and checkout is left on Enter.
	run, file := runHintFlow(t, []any{nil, 0, false, false, nil, nil}, "--dry-run")
	require.NoError(t, run.err)

	flagged := newPoppySetupHarness(t, freshPoppyTenant())
	flagged.allowWrites()
	require.NoError(t, flagged.run(freshSetupArgs(t, "--skip-doctor", "--guest-api-access", "--guest-all-scopes", "--dry-run",
		"--hint", "orders:read=changes:none,personal_data:false", "--hint", "checkout=changes:irreversible,personal_data:true")...))

	plan := func(out string) string {
		_, after, found := strings.Cut(out, "\nChecked\n")
		require.True(t, found, out)

		// The file is named by the temp directory of each run.
		return regexp.MustCompile(`\S*auth0-poppy\.json`).ReplaceAllString(after, "FILE")
	}

	assert.Equal(t, plan(flagged.out.String()), plan(run.out), "the plan, the answers and the command to apply are the same")
	assert.Contains(t, run.out, "orders:read: none")
	assert.Contains(t, run.out, "checkout: irreversible, personal data")
	assert.Contains(t, run.out, "--hint orders:read=changes:none,personal_data:false")
	assert.Contains(t, run.out, "--hint checkout=changes:irreversible,personal_data:true")
	assert.Equal(t, []string{
		"Set them now?",
		changeQuestion, personalData, "Use the same answers for the remaining 1 scope?",
		changeQuestion, personalData,
	}, run.asked)
	assert.NoFileExists(t, file, "a dry run writes nothing")
}

func TestPoppySetupHintAnswersWriteTheSameFile(t *testing.T) {
	run, file := runHintFlow(t, []any{nil, 0, false, true}, "--yes")
	require.NoError(t, run.err)

	answered, err := os.ReadFile(file)
	require.NoError(t, err)

	flagged := newPoppySetupHarness(t, freshPoppyTenant())
	flagged.allowWrites()

	args := freshSetupArgs(t, "--skip-doctor", "--guest-api-access", "--guest-all-scopes", "--yes",
		"--hint", "orders:read=changes:none,personal_data:false", "--hint", "checkout=changes:none,personal_data:false")
	require.NoError(t, flagged.run(args...))

	want, err := os.ReadFile(outOf(args))
	require.NoError(t, err)

	assert.JSONEq(t, string(want), string(answered))
	assert.Contains(t, string(answered), `"changes": "none"`)
	assert.NotContains(t, run.out, "Set the hints of", "every scope has hints, so nothing is left to set")
}

func TestPoppySetupSkippingHintsNamesTheFlag(t *testing.T) {
	run, file := runHintFlow(t, []any{false}, "--yes")
	require.NoError(t, run.err)

	assert.Equal(t, []string{"Set them now?"}, run.asked)
	assert.Contains(t, run.out, "Set the hints of 2 scopes with --hint")
	assert.Contains(t, run.out, "Agents show them before asking for approval.")
	assert.NotContains(t, run.out, "Hints  ", "no hints, no line in the answers")

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "orders:read\": {", "nothing is written for the skipped hints")
}

func TestPoppySetupHintsAreNotAskedAgain(t *testing.T) {
	t.Run("--hint answers a scope without a question", func(t *testing.T) {
		run, _ := runHintFlow(t, []any{nil, nil, nil}, "--dry-run", "--hint", "checkout=changes:none,personal_data:false")
		require.NoError(t, run.err)

		assert.Equal(t, []string{"Set it now?", changeQuestion, personalData}, run.asked, "only orders:read is asked about")
		assert.Contains(t, run.out, "checkout: none")
		assert.Contains(t, run.out, "orders:read: irreversible, personal data")
	})

	t.Run("--hint on every scope asks nothing", func(t *testing.T) {
		run, _ := runHintFlow(t, nil, "--dry-run", "--hint", "checkout=changes:none,personal_data:false", "--hint", "orders:read=changes:none,personal_data:false")
		require.NoError(t, run.err)

		assert.Empty(t, run.asked)
	})

	t.Run("a hint in the file of a configured tenant is not asked", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--guest-api-access", "--guest-all-scopes", "--out", goldenPoppyConfig(t), "--skip-doctor", "--dry-run"}

		script := &poppyScript{}
		run := h.runHintMode(t, script, args)

		require.NoError(t, run.err)
		assert.Empty(t, script.asked, "the file hints every scope")
	})

	t.Run("the reserved sign-in scope is never asked", func(t *testing.T) {
		run, _ := runHintFlow(t, make([]any, 6), "--dry-run")
		require.NoError(t, run.err)

		assert.Contains(t, strings.Join(run.asked, "|"), "Set them now?", "two scopes are asked about, not three")
		assert.NotContains(t, run.out, "poppy:browser-sign-in: ")
	})
}

func TestPoppySetupHintsCtrlC(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()

	args := freshSetupArgs(t, "--skip-doctor", "--guest-api-access", "--guest-all-scopes")
	script := &poppyScript{answers: []any{nil, terminal.InterruptErr}}

	run := h.runHintMode(t, script, args)

	require.ErrorIs(t, run.err, terminal.InterruptErr)
	assert.Contains(t, run.out, display.PoppyCancelled)
	assert.NotContains(t, run.out, "Plan:", "nothing is planned after the user cancelled")
	assert.Empty(t, h.calls, "nothing was changed")
	assert.NoFileExists(t, outOf(args))
}

func TestPoppyConfigAsksTheHints(t *testing.T) {
	newArgs := func(t *testing.T) ([]string, string) {
		out := filepath.Join(t.TempDir(), "auth0-poppy.json")

		return []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", out}, out
	}

	t.Run("the answers go to the file and are listed", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, out := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{nil, 0, false, false, nil, nil}}, args)

		require.NoError(t, run.err)

		data, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"orders:read": {`)
		assert.Regexp(t, `"changes": "none",\s+"personal_data": false`, string(data))
		assert.Contains(t, run.out, "Your answers")
		assert.Contains(t, run.out, "orders:read: none")
		assert.Contains(t, run.out, "checkout: irreversible, personal data")
		assert.Equal(t, 1, strings.Count(run.out, "Poppy config for shop.example"), "the title is printed once")
	})

	t.Run("skipping them names the flag", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, _ := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{false}}, args)

		require.NoError(t, run.err)
		assert.Contains(t, run.out, "auth0 poppy config --hint <scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>")
		assert.Contains(t, run.out, "to set the hints of 2 scopes.")
	})

	t.Run("Ctrl-C writes no file", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, out := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{terminal.InterruptErr}}, args)

		require.ErrorIs(t, run.err, terminal.InterruptErr)
		assert.Contains(t, run.out, display.PoppyCancelled)
		assert.NoFileExists(t, out)
	})
}

func TestPoppyScreenAskLine(t *testing.T) {
	t.Run("an empty answer asks again, and the error goes with the block", func(t *testing.T) {
		script := []string{"", "  ", "https://api.shop.example"}
		screen, out := newTestPoppyScreen(t, poppyAsker{
			text: func(*poppyScreen, string, string) (string, error) {
				answer := script[0]
				script = script[1:]

				return answer, nil
			},
		}, true)

		answer, err := screen.askLine("API", "The API.", "API", "")

		require.NoError(t, err)
		assert.Equal(t, "https://api.shop.example", answer)
		assert.Equal(t, 2, strings.Count(out.String(), "A value is required."))
		assert.Regexp(t, `\r\x1b\[5A\x1b\[J$`, out.String(), "blank, title, reason and the two errors, all taken back")
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		screen, out := newTestPoppyScreen(t, poppyAsker{
			text: func(*poppyScreen, string, string) (string, error) { return "", terminal.InterruptErr },
		}, true)

		_, err := screen.askLine("API", "The API.", "API", "")

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.True(t, strings.HasSuffix(out.String(), display.PoppyCancelled+"\n"))
	})

	t.Run("an answer the question refuses ends the command", func(t *testing.T) {
		screen, _ := newTestPoppyScreen(t, poppyAsker{
			text: func(*poppyScreen, string, string) (string, error) { return "https://shop.example/path", nil },
		}, false)

		var in setup.Inputs

		err := screen.askText(setup.RequiredQuestions(in)[0], &in)

		require.Error(t, err)
		assert.Equal(t, "invalid_flag", errorReason(err))
		assert.Empty(t, in.CompanyDomain)
	})

	t.Run("an answer the question takes is stored", func(t *testing.T) {
		screen, _ := newTestPoppyScreen(t, poppyAsker{
			text: func(*poppyScreen, string, string) (string, error) { return "Shop.Example", nil },
		}, false)

		var in setup.Inputs

		require.NoError(t, screen.askText(setup.RequiredQuestions(in)[0], &in))
		assert.Equal(t, "shop.example", in.CompanyDomain)
	})
}

func TestPoppyScreenConfirm(t *testing.T) {
	tests := []struct {
		name   string
		answer bool
		def    bool
	}{
		{name: "yes", answer: true},
		{name: "no", answer: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotDefault bool

			screen, out := newTestPoppyScreen(t, poppyAsker{
				confirm: func(_ *poppyScreen, _ string, def bool) (bool, error) {
					gotDefault = def

					return test.answer, nil
				},
			}, false)

			yes, err := screen.confirmAsk("Apply the plan", "It changes your tenant.", "Apply 3 changes?")

			require.NoError(t, err)
			assert.Equal(t, test.answer, yes)
			assert.False(t, gotDefault, "Enter is no")
			assert.Contains(t, out.String(), "Apply the plan\nIt changes your tenant.\n")
		})
	}
}

func TestPoppyScreenLeftOnScreen(t *testing.T) {
	tests := []struct {
		name string
		rows []string
		want int
	}{
		{name: "one row each", rows: []string{"  Question", "    one", "    two"}, want: 3},
		{name: "a long row wraps", rows: []string{"  Question", "    " + strings.Repeat("x", 100)}, want: 3},
		{name: "none", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			screen, out := newTestPoppyScreen(t, poppyAsker{}, true)

			screen.leftOnScreen(test.rows...)

			assert.Equal(t, test.want, screen.BlockRows())
			assert.Empty(t, out.String(), "counting writes nothing")
		})
	}
}

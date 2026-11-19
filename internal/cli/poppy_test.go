package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

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
				"company-domain", "api", "api-url", "app", "no-api", "no-web", "scope", "hint", "company-name", "login-uri", "out",
				"dry-run", "force", "skip-doctor", "web-unscoped", "overwrite-config", "json", "json-compact",
			},
		},
		{
			name: "poppy config",
			path: []string{"poppy", "config"},
			flags: []string{
				"company-domain", "api", "api-url", "app", "no-api", "no-web", "hint", "company-name", "out",
				"web-unscoped", "overwrite-config", "json", "json-compact",
			},
		},
		{
			name:  "poppy doctor",
			path:  []string{"poppy", "doctor"},
			flags: []string{"config", "strict", "json", "json-compact"},
		},
		{
			name: "agents register",
			path: []string{"agents", "register"},
			flags: []string{
				"api", "poppy", "name", "config", "force", "scopes", "all-scopes", "require-dpop", "json", "json-compact",
			},
		},
		{
			name:  "agents unregister",
			path:  []string{"agents", "unregister"},
			flags: []string{"force", "dry-run", "poppy", "json", "json-compact"},
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

	t.Run("agents register rejects --scopes with --all-scopes", func(t *testing.T) {
		root := newPoppyTestRoot()
		root.SetArgs([]string{"agents", "register", "--scopes", "orders:read", "--all-scopes"})

		err := root.Execute()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "scopes")
		assert.Contains(t, err.Error(), "all-scopes")
	})

	t.Run("agents has no remove command", func(t *testing.T) {
		agentsCmd, _, err := newPoppyTestRoot().Find([]string{"agents"})
		require.NoError(t, err)

		var names []string
		for _, sub := range agentsCmd.Commands() {
			names = append(names, sub.Name())
		}

		assert.ElementsMatch(t, []string{"list", "show", "create", "update", "delete", "register", "unregister"}, names)
	})

	t.Run("auth0 test poppy is not registered", func(t *testing.T) {
		testCmd, _, err := newPoppyTestRoot().Find([]string{"test"})
		require.NoError(t, err)

		for _, sub := range testCmd.Commands() {
			assert.NotEqual(t, "poppy", sub.Name())
		}
	})
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

	previous := promptAsk
	t.Cleanup(func() { promptAsk = previous })

	promptAsk.canPrompt = func(cmd *cobra.Command) bool {
		noInput, err := cmd.Root().Flags().GetBool("no-input")

		return err == nil && mode.terminal && !noInput
	}
	promptAsk.text = func(label, _ string) (string, error) {
		asked = append(asked, "text "+label)

		return "Reads the orders of the signed-in user.", nil
	}
	promptAsk.confirm = func(message string, _ bool) (bool, error) {
		switch {
		case strings.HasPrefix(message, "Set "):
			asked = append(asked, "hints")

			return false, nil
		case message == poppyWebUnscopedText:
			asked = append(asked, "web access")

			return true, nil
		case message == poppyAPIPartText, message == poppyWebPartText:
			// A person wants both parts, and the questions are not kept in the order of the others.
			return true, nil
		}

		asked = append(asked, "confirm")

		return true, nil
	}

	return &asked
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

	// More APIs and apps of the tenant, besides the server and the app.
	moreServers []*managementv3.ResourceServer
	moreApps    []*managementv3.GetClientResponseContent
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
		PersonalAgentProtocolCompanyDomains:        []string{"shop.example"},
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
	agents   *mock.MockAgentsAPI
	policies *mock.MockGuardianPolicyAPIV3
	run      func(args ...string) error

	// Runs the command in agent mode when set.
	agentMode bool

	// The Management API writes made, in order, and the error that a write of a
	// kind (tenant, api or app) returns.
	calls []string
	fail  map[string]error
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
		agents:   mock.NewMockAgentsAPI(ctrl),
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
			all := append([]*managementv3.ResourceServer{tenant.server}, tenant.moreServers...)
			if len(request.Identifiers) == 0 {
				return testServerPage(all...), nil
			}

			var found []*managementv3.ResourceServer
			for _, server := range all {
				if slices.ContainsFunc(request.Identifiers, func(identifier *string) bool { return *identifier == server.GetIdentifier() }) {
					found = append(found, server)
				}
			}

			return testServerPage(found...), nil
		}).AnyTimes()
	h.clients.EXPECT().Get(gomock.Any(), poppySetupAppID, gomock.Any()).Return(tenant.app, nil).AnyTimes()
	for _, app := range tenant.moreApps {
		h.clients.EXPECT().Get(gomock.Any(), app.GetClientID(), gomock.Any()).Return(app, nil).AnyTimes()
	}
	h.clients.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *managementv3.ListClientsRequestParameters, ...any) (*auth0.ClientListPage, error) {
			clients := []*managementv3.Client{
				{
					ClientID:             testPtr("client_muse"),
					ExternalClientID:     testPtr(poppyTestAgentURL),
					ExternalMetadataType: testPtr(managementv3.ClientExternalMetadataTypeEnumCimd),
				},
				{
					ClientID:                      tenant.app.ClientID,
					Name:                          tenant.app.Name,
					AppType:                       testPtr(managementv3.ClientAppTypeEnumRegularWeb),
					IsFirstParty:                  tenant.app.IsFirstParty,
					InitiateLoginURI:              tenant.app.InitiateLoginURI,
					AllowPersonalAgentBrowserJoin: tenant.app.AllowPersonalAgentBrowserJoin,
				},
			}

			for _, app := range tenant.moreApps {
				clients = append(clients, &managementv3.Client{
					ClientID:                      app.ClientID,
					Name:                          app.Name,
					AppType:                       app.AppType,
					IsFirstParty:                  app.IsFirstParty,
					InitiateLoginURI:              app.InitiateLoginURI,
					AllowPersonalAgentBrowserJoin: app.AllowPersonalAgentBrowserJoin,
				})
			}

			return testClientPage(clients...), nil
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

	useDoctorTestOptions(t)

	testCLI := &cli{
		renderer: renderer,
		tenant:   poppySetupTenant,
		api:      &auth0.API{CustomDomain: h.domains},
		apiv3: &auth0.APIV3{
			TenantSettings:   h.tenant,
			ResourceServerV3: h.servers,
			ClientV3:         h.clients,
			ClientGrant:      h.grants,
			Agents:           h.agents,
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

	data, err := os.ReadFile(filepath.Join("..", "poppy", "config", "testdata", "poppy.golden.json"))
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "poppy.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return path
}

func freshSetupArgs(t *testing.T, extra ...string) []string {
	t.Helper()

	return append([]string{
		"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
		"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json",
		"--out", filepath.Join(t.TempDir(), "poppy.json"),
	}, extra...)
}

func TestPoppySetupPlan(t *testing.T) {
	t.Run("dry run prints the plan and applies nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run(freshSetupArgs(t, "--dry-run")...)

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
		assert.NotContains(t, text, "Guest grant")
		assert.Contains(t, text, "File\n")
		assert.Contains(t, text, "To apply:  auth0 poppy setup --tenant "+poppySetupTenant+" --company-domain shop.example --api "+poppyTestAPI)
		assert.NotContains(t, text, "--dry-run", "the command that applies the plan is not a dry run")
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
			"--out", goldenPoppyConfig(t))

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
			wantReason: "missing_required_flags",
			wantText:   "missing required flags: --company-domain, --api-url " + poppyTestAPI + "=<openapi url>",
		},
		{
			name:       "the API is required without a terminal",
			args:       []string{"setup", "--company-domain", "shop.example", "--app", poppySetupAppID},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_required_flags",
			wantText:   "missing required flags: --api",
		},
		{
			name:       "the app is required without a terminal",
			args:       []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI},
			tenant:     freshPoppyTenant(),
			wantReason: "missing_required_flags",
			wantText:   "missing required flags: --app",
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
			wantReason: "missing_required_flags",
			wantText:   "missing required flags: --api-url " + poppyTestAPI + "=<openapi url>",
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
			wantReason: "missing_required_flags",
			wantText:   "--scope 'checkout=<description>'",
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

// TestPoppyIssuerMismatchAsksNothing checks that a file of another issuer is
// refused before the first question, not after the last one.
func TestPoppyIssuerMismatchAsksNothing(t *testing.T) {
	// The golden file has the issuer of a custom domain, and these tenants have none.
	configured := func() poppyTenantState { state := configuredPoppyTenant(); state.domains = nil; return state }
	fresh := func() poppyTenantState { state := freshPoppyTenant(); state.domains = nil; return state }

	setupArgs := func(t *testing.T, extra ...string) []string {
		return freshSetupArgs(t, append([]string{"--out", goldenPoppyConfig(t), "--skip-doctor"}, extra...)...)
	}
	// Hints that the file lacks are what config would ask about.
	configArgs := func(t *testing.T, extra ...string) []string {
		path := goldenPoppyConfig(t)

		cfg, err := config.Read(path)
		require.NoError(t, err)

		for name, scope := range cfg.Scopes {
			scope.Hints = nil
			cfg.Scopes[name] = scope
		}
		require.NoError(t, config.Write(path, cfg))

		return append([]string{
			"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", path,
		}, extra...)
	}

	tests := []struct {
		name   string
		tenant func() poppyTenantState
		args   func(t *testing.T, extra ...string) []string
	}{
		{name: "setup", tenant: fresh, args: setupArgs},
		{name: "config", tenant: configured, args: configArgs},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("no question is asked", func(t *testing.T) {
				h := newPoppySetupHarness(t, test.tenant())
				h.allowWrites()

				run := h.runPromptMode(t, modeTerminal, test.args(t))

				assert.Empty(t, run.asked, "questions asked")

				reason, _ := decodeCLIError(t, run.err)
				assert.Equal(t, "issuer_mismatch", reason)
				assert.ErrorIs(t, run.err, config.ErrIssuerMismatch)
				assert.Contains(t, run.err.Error(), "Pass --overwrite-config to overwrite it")
			})

			t.Run("--overwrite-config goes on", func(t *testing.T) {
				h := newPoppySetupHarness(t, test.tenant())
				h.allowWrites()

				run := h.runPromptMode(t, modeTerminal, test.args(t, "--overwrite-config"))

				require.NoError(t, run.err)
			})
		})
	}
}

func TestPoppyMissingValuesError(t *testing.T) {
	tests := []struct {
		name        string
		fields      []string
		in          setup.Inputs
		opts        setup.CommandOptions
		wantReason  string
		wantText    string
		wantFlags   []string
		wantExample string
	}{
		{
			name:        "the company name",
			fields:      []string{"organization.name"},
			wantReason:  "missing_required_flags",
			wantText:    "missing required flags: --company-name",
			wantFlags:   []string{"--company-name"},
			wantExample: "auth0 poppy setup --company-name <name>",
		},
		{
			name:        "the OpenAPI URL of an API",
			fields:      []string{"apis[https://api.shop.example].url"},
			wantReason:  "missing_required_flags",
			wantText:    "missing required flags: --api-url https://api.shop.example=<openapi url>",
			wantFlags:   []string{"--api-url https://api.shop.example=<openapi url>"},
			wantExample: "auth0 poppy setup --api-url https://api.shop.example=<openapi url>",
		},
		{
			name:        "the company domain",
			fields:      []string{"organization.domain"},
			wantReason:  "missing_required_flags",
			wantFlags:   []string{"--company-domain"},
			wantExample: "auth0 poppy setup --company-domain <domain>",
		},
		{
			name:        "several values, with the flags that were passed",
			fields:      []string{"apis[x].url", "organization.name"},
			in:          setup.Inputs{CompanyDomain: "shop.example", APIs: []string{"x"}},
			opts:        setup.CommandOptions{Tenant: "t.auth0.com"},
			wantReason:  "missing_required_flags",
			wantFlags:   []string{"--company-name", "--api-url x=<openapi url>"},
			wantExample: "auth0 poppy setup --tenant t.auth0.com --company-domain shop.example --api x --company-name <name> --api-url x=<openapi url>",
		},
		{
			name:        "the company domain comes before the name and the OpenAPI URL",
			fields:      []string{"apis[x].url", "organization.domain", "organization.name"},
			wantReason:  "missing_required_flags",
			wantFlags:   []string{"--company-domain", "--company-name", "--api-url x=<openapi url>"},
			wantExample: "auth0 poppy setup --company-domain <domain> --company-name <name> --api-url x=<openapi url>",
		},
		{
			name:        "the config command",
			fields:      []string{"organization.name"},
			in:          setup.Inputs{CompanyDomain: "shop.example", Scopes: map[string]string{"a": "b"}},
			opts:        setup.CommandOptions{Config: true},
			wantReason:  "missing_required_flags",
			wantFlags:   []string{"--company-name"},
			wantExample: "auth0 poppy config --company-domain shop.example --company-name <name>",
		},
		{
			name:       "an API description, which no flag sets",
			fields:     []string{"apis[x].description"},
			wantReason: "missing_config_values",
			wantText:   "missing required values: apis[x].description. No flag sets apis[x].description: the description of an API is its name in Auth0",
		},
		{
			name:        "a value with a flag and one without",
			fields:      []string{"apis[x].description", "organization.name"},
			wantReason:  "missing_required_flags",
			wantText:    "missing required flags: --company-name. No flag sets apis[x].description",
			wantFlags:   []string{"--company-name"},
			wantExample: "auth0 poppy setup --company-name <name>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := poppyMissingValuesError(&config.MissingFieldsError{Fields: test.fields}, test.in, test.opts)

			assert.Equal(t, test.wantReason, errorReason(err))
			assert.Contains(t, err.Error(), test.wantText)
			assert.NotContains(t, err.Error(), "Pass --api-url as")

			if test.wantReason != "missing_required_flags" {
				return
			}

			assert.Equal(t, "usage", errorClass(err))

			_, details := decodeCLIError(t, err)

			var flags []string
			for _, entry := range details["flags"].([]any) {
				flags = append(flags, entry.(map[string]any)["flag"].(string))
				assert.NotEmpty(t, entry.(map[string]any)["reason"])
			}

			assert.Equal(t, test.wantFlags, flags)
			assert.Equal(t, test.wantExample, details["example"])
		})
	}

	t.Run("setup error maps it the same way, without the flags passed", func(t *testing.T) {
		err := poppySetupError(&config.MissingFieldsError{Fields: []string{"apis[x].url"}})

		assert.Equal(t, "missing_required_flags", errorReason(err))
		assert.EqualError(t, err, "missing required flags: --api-url x=<openapi url>")
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
			wantText:   "bad",
		},
		{
			name:       "an unready tenant",
			err:        &setup.NotReadyError{Problems: []string{"a"}, Fix: "b"},
			wantReason: "poppy_not_ready",
			wantText:   "the tenant is not set up for Poppy:\n  - a\n  FIX  b",
		},
		{
			name:       "a wrapped issuer mismatch",
			err:        errors.Join(config.ErrIssuerMismatch, errors.New("detail")),
			wantReason: "issuer_mismatch",
			wantText:   "Pass --overwrite-config to overwrite it",
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

	t.Run("an input error has a one line message and its fix in the details", func(t *testing.T) {
		err := poppySetupError(&setup.InputError{Message: "bad", Reason: "invalid_flag", Fix: "do this"})

		assert.EqualError(t, err, "bad")

		_, details := decodeCLIError(t, err)
		assert.Equal(t, map[string]any{"fix": "do this"}, details)

		var hint apiResponseHintError
		require.ErrorAs(t, err, &hint)
		assert.Equal(t, "FIX  do this", hint.hint, "the renderer prints the fix under the message")
	})

	t.Run("the candidates of an ambiguous input are in the details next to the fix", func(t *testing.T) {
		tests := []struct {
			name  string
			input *setup.InputError
		}{
			{
				name: "an ambiguous app",
				input: &setup.InputError{
					Reason: setup.ReasonAmbiguousApp, Message: "several apps", Fix: "Pass --app <client ID or name> to name the web app.",
					Candidates: []string{"client-a", "client-b"},
				},
			},
			{
				name: "an ambiguous API",
				input: &setup.InputError{
					Reason: setup.ReasonAmbiguousAPI, Message: "several APIs", Fix: "Pass --api <identifier> to name the API.",
					Candidates: []string{"https://api.shop.example", "https://api.shop.example/v2"},
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				err := poppySetupError(test.input)

				assert.Equal(t, test.input.Reason, errorReason(err))

				_, details := decodeCLIError(t, err)
				assert.Equal(t, map[string]any{"fix": test.input.Fix, "candidates": toAnySlice(test.input.Candidates)}, details)

				var hint apiResponseHintError
				require.ErrorAs(t, err, &hint)
				assert.Equal(t, "FIX  "+test.input.Fix, hint.hint)
			})
		}
	})

	t.Run("candidates without a fix are in the details and have no hint", func(t *testing.T) {
		err := poppySetupError(&setup.InputError{Message: "bad", Reason: setup.ReasonAmbiguousApp, Candidates: []string{"client-a", "client-b"}})

		_, details := decodeCLIError(t, err)
		assert.Equal(t, map[string]any{"candidates": []any{"client-a", "client-b"}}, details)
		assert.False(t, errors.As(err, new(apiResponseHintError)))
	})

	t.Run("a fix over several lines keeps its lines under the first", func(t *testing.T) {
		err := poppySetupError(&setup.InputError{Message: "bad", Reason: "invalid_flag", Fix: "Pass one of:\n         --a\n         --b"})

		var hint apiResponseHintError
		require.ErrorAs(t, err, &hint)
		assert.Equal(t, "FIX  Pass one of:\n           --a\n           --b", hint.hint)
	})

	t.Run("an input error without a fix has no details and no hint", func(t *testing.T) {
		err := poppySetupError(&setup.InputError{Message: "bad", Reason: "invalid_flag"})

		assert.EqualError(t, err, "bad")
		assert.Nil(t, errorDetails(err))
		assert.False(t, errors.As(err, new(apiResponseHintError)))
	})
}

// toAnySlice converts strings to the []any a decoded JSON list has.
func toAnySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}

	return out
}

// outOf returns the config file path in the arguments.
func outOf(args []string) string {
	return args[slices.Index(args, "--out")+1]
}

func TestPoppySetupApply(t *testing.T) {
	t.Run("applies the plan, writes the file last and runs doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()
		args := freshSetupArgs(t, "--force")

		err := h.run(args...)

		require.NoError(t, err, "a doctor failure does not fail setup")
		assert.Equal(t, []string{"tenant", "api", "app"}, h.calls)

		cfg, err := config.Read(outOf(args))
		require.NoError(t, err)
		assert.Equal(t, "shop.example", cfg.Organization.Domain)

		text := h.out.String()
		assert.Contains(t, text, "Poppy setup for shop.example")
		assert.Contains(t, text, "Applying 12 changes\n")
		// Setup has a login, so doctor reads the tenant. The website is usually not
		// deployed yet, so only the checks of the tenant run (D8 to D14). The mocks
		// serve the tenant as it was before the plan, without the company domain, and
		// the site cannot be reached to tell which tenant serves it. So the doctor
		// takes the tenant for another one and skips its checks.
		assert.NotContains(t, text, "FAIL D")
		assert.NotContains(t, text, ".well-known")
		assert.NotContains(t, text, "- skipped", "the checks that did not run are not listed after setup")
		assert.NotContains(t, text, "skipped (")
		assert.Contains(t, text, "0 passed, 7 skipped.")
		assert.Contains(t, text, "Checking your setup (auth0 poppy doctor)")
		assert.Contains(t, text, "written\n")
		assert.Contains(t, text, "Only the checks of your tenant ran, because your website is usually not deployed yet. Once it is, run `auth0 poppy doctor shop.example` to check everything.")
		assert.NotContains(t, text, "Deploy the config file with your app")
		assert.NotContains(t, text, "to see every check")
		assert.Contains(t, text, "Next\n  1. Deploy ")
		assert.Empty(t, h.messages.String(), "the result is on stdout")
	})

	t.Run("no client grant is created", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--force", "--skip-doctor")...))

		assert.Equal(t, []string{"tenant", "api", "app"}, h.calls)
	})

	t.Run("skip doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--force", "--skip-doctor")...))

		assert.NotContains(t, h.out.String(), "Checking your setup")
		assert.NotContains(t, h.out.String(), ".well-known")
		assert.NotContains(t, h.out.String(), "Deploy the config file")
	})

	t.Run("a tenant that is set up has nothing to change and still runs doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", goldenPoppyConfig(t)}

		require.NoError(t, h.run(args...))

		assert.Empty(t, h.calls)
		assert.Contains(t, h.out.String(), "Nothing to change")
		assert.NotContains(t, h.out.String(), "Applying", "nothing is applied")
		assert.Contains(t, h.out.String(), "Checking your setup")
		assert.Contains(t, h.out.String(), "Only the checks of your tenant ran")
		assert.NotContains(t, h.out.String(), ".well-known/poppy.json", "the website checks do not run after setup")

		h = newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, h.run(append(args, "--skip-doctor")...))
		assert.NotContains(t, h.out.String(), "Checking your setup")
	})

	t.Run("an empty plan asks for nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", goldenPoppyConfig(t), "--skip-doctor"}

		require.NoError(t, h.run(args...), "no terminal and no --force")
	})
}

func TestPoppySetupConfirmation(t *testing.T) {
	t.Run("without a terminal and without --yes nothing changes", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		args := freshSetupArgs(t)

		err := h.run(args...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "confirmation_required", reason)
		assert.Contains(t, err.Error(), "--force")
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

	t.Run("dry run never writes, even with --force", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		args := freshSetupArgs(t, "--dry-run", "--force")

		require.NoError(t, h.run(args...), "the mocks have no write calls")

		assert.NoFileExists(t, outOf(args))
		assert.NotContains(t, h.messages.String(), "Doctor")
	})
}

func TestPoppySetupJSON(t *testing.T) {
	t.Run("plan, applied and doctor", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--force", "--json")...))

		var result struct {
			Plan    setup.Plan     `json:"plan"`
			Applied []setup.Change `json:"applied"`
			Doctor  *doctor.Report `json:"doctor"`
		}
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &result), "one object")
		require.Len(t, result.Plan.Changes, 4)
		assert.Equal(t, result.Plan.Changes, result.Applied, "applied has the same change objects as the plan")
		require.NotNil(t, result.Doctor)
		assert.Equal(t, "shop.example", result.Doctor.Domain)
		assert.True(t, result.Doctor.TenantOnly, "the website is usually not deployed yet, so only the tenant is checked")
		assert.Zero(t, result.Doctor.ExitCode())
		assert.Equal(t, 7, result.Doctor.Totals.Skipped, "the mocks serve a tenant the doctor takes for another one")
		assert.Zero(t, result.Doctor.Totals.Fail)

		for _, res := range result.Doctor.Results {
			assert.Contains(t, []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"}, res.ID, "no website check is in the report")
		}
	})

	t.Run("skip doctor leaves the doctor out", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		require.NoError(t, h.run(freshSetupArgs(t, "--force", "--json-compact", "--skip-doctor")...))

		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &result))
		assert.Contains(t, result, "plan")
		assert.Contains(t, result, "applied")
		assert.NotContains(t, result, "doctor")
	})

	t.Run("an empty plan applies nothing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())

		require.NoError(t, h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--out", goldenPoppyConfig(t), "--skip-doctor", "--json"))

		assert.JSONEq(t, `[]`, string(jsonField(t, h.out.Bytes(), "applied")))
	})

	t.Run("the plan always has its warnings and app_first_party", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())

		require.NoError(t, h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--out", goldenPoppyConfig(t), "--skip-doctor", "--json"))

		plan := jsonField(t, h.out.Bytes(), "plan")
		assert.JSONEq(t, `[]`, string(jsonField(t, plan, "warnings")), "a plan without warnings has a list, not null")
		assert.JSONEq(t, `true`, string(jsonField(t, plan, "app_first_party")))
	})

	t.Run("a dry run prints the same plan", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())

		require.NoError(t, h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--out", goldenPoppyConfig(t), "--dry-run", "--json"))

		assert.JSONEq(t, `[]`, string(jsonField(t, h.out.Bytes(), "warnings")))
		assert.JSONEq(t, `true`, string(jsonField(t, h.out.Bytes(), "app_first_party")))
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
			wantCalls: []string{"tenant", "api"}, wantPartial: true, wantMessage: "auth0 login --scopes update:resource_servers", wantReason: "insufficient_scope",
		},
		{
			name: "any other 403", fail: "app", err: forbidden(`{"errorCode":"poppy_beta_off"}`),
			wantCalls: []string{"tenant", "api", "app"}, wantPartial: true, wantMessage: "the tenant may lack Agent as Principal or the Poppy beta",
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
				args := freshSetupArgs(t, append([]string{"--force"}, mode.flags...)...)

				err := h.run(args...)

				require.Error(t, err)
				assert.Equal(t, test.wantCalls, h.calls, "stops at the first failure")
				assert.Contains(t, err.Error(), test.wantMessage)
				assert.NoFileExists(t, outOf(args), "the file is written last")
				assert.NotContains(t, h.out.String(), "Checking your setup", "doctor runs only after a full apply")

				var partial *setup.PartialError
				isPartial := errors.As(err, &partial)

				// A person gets the cause only: the step list says what was applied and how
				// to continue. In both modes the error is the cause, with the account in its details.
				assert.False(t, isPartial)
				assert.NotContains(t, err.Error(), "stopped partway")
				assert.NotContains(t, err.Error(), "Running the command again continues")

				if !test.wantPartial {
					assert.NotEqual(t, "partial_failure", errorReason(err))

					return
				}

				if test.wantReason == "" {
					assert.Equal(t, "partial_failure", errorReason(err))
				} else {
					assert.Equal(t, test.wantReason, errorReason(err))
				}

				details := decodeDetails(t, err)
				assert.Len(t, details["applied"], len(test.wantCalls)-1)
				assert.NotEmpty(t, details["failed"])
				assert.IsType(t, []any{}, details["remaining"], "a list even when nothing is left")
				assert.Contains(t, details["next"], "auth0 poppy setup --tenant "+poppySetupTenant+" --company-domain shop.example --api "+poppyTestAPI)
				assert.NotContains(t, details["next"], "--dry-run")

				if len(mode.flags) == 0 {
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
			"setup":         {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag, "--force", "--skip-doctor") }},
			"setup dry run": {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag, "--dry-run") }},
			"setup refused": {freshPoppyTenant, func(t *testing.T) []string { return freshSetupArgs(t, flag) }},
			"config": {configuredPoppyTenant, func(t *testing.T) []string {
				return []string{"config", flag, "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
					"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", filepath.Join(t.TempDir(), "poppy.json")}
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

		err := h.run(freshSetupArgs(t, "--force")...)

		require.Error(t, err)

		out := h.out.String()
		assert.Contains(t, out, "Applying 12 changes\n")
		assert.Contains(t, out, "  ok Tenant settings")
		assert.Contains(t, out, "5 changes")
		assert.Contains(t, out, "  x  App Shop Web")
		assert.Contains(t, out, "failed\n")
		assert.Contains(t, out, "boom")
		assert.Contains(t, out, "not run")
		assert.Contains(t, out, "Running the command again continues from here.\nTo rerun:  auth0 poppy setup --tenant "+poppySetupTenant+" --company-domain shop.example --api "+poppyTestAPI)
		assert.Contains(t, out, " --force")
		assert.NotContains(t, out, " --yes")
		assert.NotContains(t, out, "--dry-run")
		assert.Empty(t, h.messages.String())
	})

	t.Run("the output flags print no block", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			h := newPoppySetupHarness(t, freshPoppyTenant())
			h.allowWrites()
			h.fail["app"] = errors.New("boom")

			err := h.run(freshSetupArgs(t, "--force", flag)...)

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
	args := freshSetupArgs(t, "--force")

	err = h.run(args...)

	require.Error(t, err)
	assert.Equal(t, "missing_scopes", errorReason(err))
	assert.Contains(t, err.Error(), "auth0 login --scopes ")
	assert.Contains(t, err.Error(), "update:tenant_settings")
	assert.Empty(t, h.calls, "nothing is changed")
	assert.NoFileExists(t, outOf(args))

	h = newPoppySetupHarness(t, freshPoppyTenant())
	require.NoError(t, h.run(freshSetupArgs(t, "--dry-run")...), "a dry run needs only the read scopes")

	// The scopes are checked before the first question, not after the last.
	asked := installPoppyAsker(t, modeTerminal)
	h = newPoppySetupHarness(t, freshPoppyTenant())

	err = h.run("setup", "--force")

	require.Error(t, err)
	assert.Equal(t, "missing_scopes", errorReason(err))
	assert.Contains(t, err.Error(), "update:resource_servers")
	assert.Empty(t, *asked, "no question was asked")
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

			reason, details := decodeCLIError(t, err)
			assert.Equal(t, "missing_required_flags", reason)
			assert.EqualError(t, err, "missing required flags: --scope 'checkout=<description>'")
			assert.Equal(t, []any{map[string]any{
				"flag":   "--scope 'checkout=<description>'",
				"reason": poppyTestAPI + " has no description for checkout",
			}}, details["flags"])
			assert.Contains(t, details["example"], "auth0 poppy setup --tenant "+poppySetupTenant+" --company-domain shop.example --api "+poppyTestAPI)
			assert.True(t, strings.HasSuffix(details["example"].(string), " --scope 'checkout=<description>'"), details["example"])
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

			screen, out := newTestPromptScreen(t, promptAsker{
				text: func(label, _ string) (string, error) {
					labels = append(labels, label)
					if test.askErr != nil {
						return "", test.askErr
					}

					answer := test.answers[next]
					next++

					return answer, nil
				},
			})

			err := askPoppyQuestions(screen, &in, setup.ScopeQuestions(state, in))

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

	asks := []string{"hints", "web access", "confirm"}

	checkPoppySetupPromptRows(t, freshPoppyTenant(), []string{"--skip-doctor"}, []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: plan, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: plan, reason: "confirmation_required", errText: "--force"},
		{name: "piped", mode: modePiped, text: plan, reason: "confirmation_required", errText: "--force"},
		{name: "piped with --json", mode: modePipedJSON, reason: "confirmation_required", errText: "--force"},
		{name: "agent mode", mode: modeAgent, reason: "confirmation_required", errText: "--force"},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, message: plan, json: true},
	})
}

func TestPoppySetupPromptRuleScopeDescription(t *testing.T) {
	const (
		plan    = "Poppy setup for shop.example"
		missing = "missing required flags: --scope 'checkout=<description>'"
	)

	asks := []string{"text Text", "hints", "web access", "confirm"}

	tenant := freshPoppyTenant()
	tenant.server.Scopes[1].Description = nil

	checkPoppySetupPromptRows(t, tenant, []string{"--skip-doctor"}, []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: plan, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "missing_required_flags", errText: missing},
		{name: "piped", mode: modePiped, reason: "missing_required_flags", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "missing_required_flags", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "missing_required_flags", errText: missing},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, message: plan, json: true},
	})
}

func TestPoppySetupCompanyName(t *testing.T) {
	const plan = "Poppy setup for shop.example"

	unnamed := freshPoppyTenant()
	unnamed.settings.FriendlyName = nil

	asks := []string{"text Name", "hints", "web access", "confirm"}
	missing := "missing required flags: --company-name"

	t.Run("asked in a terminal, listed without one", func(t *testing.T) {
		checkPoppySetupPromptRows(t, unnamed, []string{"--skip-doctor"}, []poppyPromptRow{
			{name: "terminal", mode: modeTerminal, asked: asks, text: "Applying"},
			{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: plan, json: true},
			{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "missing_required_flags", errText: missing},
			{name: "piped", mode: modePiped, reason: "missing_required_flags", errText: missing},
			{name: "agent mode", mode: modeAgent, reason: "missing_required_flags", errText: missing},
		})
	})

	t.Run("--company-name answers it", func(t *testing.T) {
		checkPoppySetupPromptRows(t, unnamed, []string{"--skip-doctor", "--company-name", "Shop Inc"}, []poppyPromptRow{
			{name: "terminal", mode: modeTerminal, asked: []string{"hints", "web access", "confirm"}, text: "Applying"},
			{name: "piped", mode: modePiped, reason: "confirmation_required", errText: "--force", text: plan},
		})
	})

	t.Run("the error has the flag, why, and a ready command", func(t *testing.T) {
		h := newPoppySetupHarness(t, unnamed)
		args := freshSetupArgs(t)

		err := h.run(args...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.Equal(t, []any{map[string]any{"flag": "--company-name", "reason": "The name agents show for your company."}}, details["flags"])
		assert.Equal(t, "auth0 poppy setup --tenant "+poppySetupTenant+" --company-domain shop.example --api "+poppyTestAPI+
			" --api-url "+poppyTestAPI+"="+poppyTestAPI+"/openapi.json --app "+poppySetupAppID+" --out "+args[len(args)-1]+" --company-name <name>", details["example"])
		assert.Empty(t, h.calls)
	})

	t.Run("the name, the OpenAPI URL and a scope description come in one round", func(t *testing.T) {
		tenant := freshPoppyTenant()
		tenant.settings.FriendlyName = nil
		tenant.server.Scopes[1].Description = nil
		h := newPoppySetupHarness(t, tenant)

		err := h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--out", filepath.Join(t.TempDir(), "poppy.json"))

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.EqualError(t, err, "missing required flags: --company-name, --api-url "+poppyTestAPI+"=<openapi url>, --scope 'checkout=<description>'")

		flags := details["flags"].([]any)
		require.Len(t, flags, 3)
		assert.Equal(t, poppyTestAPI+" has no description for checkout", flags[2].(map[string]any)["reason"])
		assert.True(t, strings.HasSuffix(details["example"].(string),
			" --company-name <name> --api-url "+poppyTestAPI+"=<openapi url> --scope 'checkout=<description>'"), details["example"])
		assert.Empty(t, h.calls)
	})
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
			screen, out := newTestPromptScreen(t, script.asker(t))
			screen.ForDomain("shop.example")

			in := setup.Inputs{}
			unhinted, err := askPoppyHints(screen, test.scopes, &in)

			assert.Equal(t, test.wantAsked, script.asked)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.True(t, strings.HasSuffix(out.String(), display.PromptCancelled+"\n"), "the last line says nothing was changed:\n%s", out.String())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantUnhinted, unhinted)
			assert.Equal(t, test.wantHints, in.Hints)
			assert.NotContains(t, out.String(), display.PromptCancelled)
			assert.Empty(t, script.answers, "every scripted answer was used")
		})
	}
}

func TestAskPoppyHintsScreen(t *testing.T) {
	script := &poppyScript{answers: []any{nil, 0, false, false, nil, nil}}
	screen, out := newTestPromptScreen(t, script.asker(t))
	screen.ForDomain("shop.example")

	_, err := askPoppyHints(screen, hintScopes[:2], &setup.Inputs{})
	require.NoError(t, err)

	want := ansi.Bold("Poppy setup for shop.example") + strings.Repeat(" ", 30) + "tenant mock.auth0.test\n" +
		"\n" +
		"Hints for your scopes\n" +
		"2 scopes have no hints yet. Agents show them to the user before asking for approval.\n" +
		"\n" +
		"Hints for orders:read on https://api.shop.example  (1 of 2)\n" +
		"See your orders and their status.\n" +
		"\n" +
		"Hints for checkout on https://api.shop.example  (2 of 2)\n" +
		"Place orders and pay with your saved card.\n"

	assert.Equal(t, want, stripEscapes(out.String()), "the title of the run is printed once, above the first question, and no block is taken back")
}

// stripEscapes removes the escape sequences of the colors and of the cursor.
var escapes = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\r`)

func stripEscapes(s string) string { return escapes.ReplaceAllString(s, "") }

func TestAskPoppyHintsBlocks(t *testing.T) {
	t.Run("nothing is erased", func(t *testing.T) {
		script := &poppyScript{answers: []any{nil, nil, nil, false, nil, nil}}
		screen, out := newTestPromptScreen(t, script.asker(t))
		screen.ForDomain("shop.example")

		_, err := askPoppyHints(screen, hintScopes[:2], &setup.Inputs{})
		require.NoError(t, err)

		assert.NotContains(t, out.String(), "\x1b[J")
		assert.NotRegexp(t, `\x1b\[\d+A`, out.String())
		assert.Equal(t, 1, strings.Count(out.String(), "Poppy setup for shop.example"), "the title of the run is printed once")
		assert.NotContains(t, out.String(), display.PromptCancelled)
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		script := &poppyScript{answers: []any{nil, terminal.InterruptErr}}
		screen, out := newTestPromptScreen(t, script.asker(t))
		screen.ForDomain("shop.example")

		_, err := askPoppyHints(screen, hintScopes, &setup.Inputs{})

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.True(t, strings.HasSuffix(out.String(), display.PromptCancelled+"\n"))
		assert.Equal(t, 1, strings.Count(out.String(), display.PromptCancelled))
		assert.Equal(t, 1, strings.Count(out.String(), "Poppy setup for shop.example"), "the title of the run stays")
	})
}

// hintFlow runs setup in a terminal against a fresh tenant with the answers
// scripted for the questions about hints. The questions that are not about hints
// are not expected.
func runHintFlow(t *testing.T, answers []any, extra ...string) (poppyPromptRun, string) {
	t.Helper()

	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()

	args := freshSetupArgs(t, append([]string{"--skip-doctor"}, extra...)...)

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
	previous := promptAsk.confirm
	promptAsk.confirm = func(message string, def bool) (bool, error) {
		if strings.HasPrefix(message, "Set ") || strings.HasPrefix(message, "Use the same") || strings.HasPrefix(message, "Does it") {
			return scripted.confirm(message, def)
		}

		return previous(message, def)
	}
	promptAsk.choose = scripted.choose
	promptAsk.interrupt = func(err error) error { return err }

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
	require.NoError(t, flagged.run(freshSetupArgs(t, "--skip-doctor", "--dry-run",
		"--hint", "orders:read=changes:none,personal_data:false", "--hint", "checkout=changes:irreversible,personal_data:true")...))

	plan := func(out string) string {
		_, after, found := strings.Cut(out, "\nChecked\n")
		require.True(t, found, out)

		// The file is named by the temp directory of each run.
		return regexp.MustCompile(`\S*poppy\.json`).ReplaceAllString(after, "FILE")
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
	run, file := runHintFlow(t, []any{nil, 0, false, true}, "--force")
	require.NoError(t, run.err)

	answered, err := os.ReadFile(file)
	require.NoError(t, err)

	flagged := newPoppySetupHarness(t, freshPoppyTenant())
	flagged.allowWrites()

	args := freshSetupArgs(t, "--skip-doctor", "--force",
		"--hint", "orders:read=changes:none,personal_data:false", "--hint", "checkout=changes:none,personal_data:false")
	require.NoError(t, flagged.run(args...))

	want, err := os.ReadFile(outOf(args))
	require.NoError(t, err)

	assert.JSONEq(t, string(want), string(answered))
	assert.Contains(t, string(answered), `"changes": "none"`)
	assert.NotContains(t, run.out, "Set the hints of", "every scope has hints, so nothing is left to set")
}

func TestPoppySetupSkippingHintsNamesTheFlag(t *testing.T) {
	run, file := runHintFlow(t, []any{false}, "--force")
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
			"--out", goldenPoppyConfig(t), "--skip-doctor", "--dry-run"}

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

	args := freshSetupArgs(t, "--skip-doctor")
	script := &poppyScript{answers: []any{nil, terminal.InterruptErr}}

	run := h.runHintMode(t, script, args)

	require.ErrorIs(t, run.err, terminal.InterruptErr)
	assert.Contains(t, run.errOut, display.PromptCancelled)
	assert.NotContains(t, run.out, "Plan:", "nothing is planned after the user cancelled")
	assert.Empty(t, h.calls, "nothing was changed")
	assert.NoFileExists(t, outOf(args))
}

// With a JSON format, a person at a terminal still sees the blocks of the
// questions and the plan, but they are on stderr, so that stdout is the JSON
// document and nothing else.
func TestPoppyJSONKeepsStdoutClean(t *testing.T) {
	setupArgs := func(t *testing.T) []string { return freshSetupArgs(t, "--skip-doctor") }

	tests := []struct {
		name string
		mode poppyPromptMode
		run  func(t *testing.T, mode poppyPromptMode) poppyPromptRun
		// The text on stderr that a person reads before answering.
		wantErr []string
	}{
		{
			name: "poppy setup",
			mode: modeTerminalJSON,
			run: func(t *testing.T, mode poppyPromptMode) poppyPromptRun {
				h := newPoppySetupHarness(t, freshPoppyTenant())
				h.allowWrites()

				return h.runPromptMode(t, mode, setupArgs(t))
			},
			wantErr: []string{"Poppy setup for shop.example", "Hints for your scopes", "Apply the plan"},
		},
		{
			name: "poppy config",
			mode: modeTerminalJSON,
			run: func(t *testing.T, mode poppyPromptMode) poppyPromptRun {
				h := newPoppySetupHarness(t, configuredPoppyTenant())
				args := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
					"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", filepath.Join(t.TempDir(), "poppy.json")}

				return h.runPromptMode(t, mode, args)
			},
			wantErr: []string{"Poppy config for shop.example", "Hints for your scopes"},
		},
		{
			name: "agents register",
			mode: modeAgentPrompting,
			run: func(t *testing.T, mode poppyPromptMode) poppyPromptRun {
				h := newAgentsHarness(t)
				h.allowRegistration()

				return h.runPromptMode(t, mode, []string{"register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--config", filepath.Join(t.TempDir(), "none.json")})
			},
			wantErr: []string{"Agent metadata", "This will", "Register the agent"},
		},
		{
			name: "agents unregister",
			mode: modeTerminalJSON,
			run: func(t *testing.T, mode poppyPromptMode) poppyPromptRun {
				h := newAgentsHarness(t)
				h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
				h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil).AnyTimes()
				h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil).AnyTimes()
				h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil).AnyTimes()

				return h.runPromptMode(t, mode, []string{"unregister", poppyTestAgentURL})
			},
			wantErr: []string{"This will remove:", "Unregister the agent"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := test.run(t, test.mode)

			require.NoError(t, run.err)
			assert.True(t, json.Valid([]byte(run.out)), "stdout is one JSON document and nothing else:\n%s", run.out)

			for _, want := range test.wantErr {
				assert.Contains(t, stripEscapes(run.errOut), want)
			}
		})
	}
}

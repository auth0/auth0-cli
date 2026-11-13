package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	poppyagents "github.com/auth0/auth0-cli/internal/agents/poppy"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
)

type agentsHarness struct {
	clients     *mock.MockClientAPIV3
	agents      *mock.MockAgentsAPI
	grants      *mock.MockClientGrantAPIV3
	servers     *mock.MockResourceServerAPIV3
	credentials *mock.MockClientCredentialAPIV3
	out         *bytes.Buffer
	messages    *bytes.Buffer
	json        bool
	agentMode   bool
	// AppRequiresDPoP is the DPoP state of the app that registration reads back.
	appRequiresDPoP bool
	renderer        *display.Renderer
	cli             *cli
	root            func(args ...string) error
}

func newAgentsHarness(t *testing.T) *agentsHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &agentsHarness{
		clients:     mock.NewMockClientAPIV3(ctrl),
		agents:      mock.NewMockAgentsAPI(ctrl),
		grants:      mock.NewMockClientGrantAPIV3(ctrl),
		servers:     mock.NewMockResourceServerAPIV3(ctrl),
		credentials: mock.NewMockClientCredentialAPIV3(ctrl),
		out:         &bytes.Buffer{},
		messages:    &bytes.Buffer{},
	}

	renderer := display.NewRenderer()
	renderer.ResultWriter = h.out
	renderer.MessageWriter = h.messages

	testCLI := &cli{
		renderer: renderer,
		apiv3: &auth0.APIV3{
			ClientV3:         h.clients,
			Agents:           h.agents,
			ClientGrant:      h.grants,
			ResourceServerV3: h.servers,
			ClientCredential: h.credentials,
		},
	}

	h.renderer = renderer
	h.cli = testCLI

	h.root = func(args ...string) error {
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
		// Registering the flags resets the output options to their defaults.
		testCLI.json = h.json
		root.SetArgs(append([]string{"agents"}, args...))
		root.SetContext(context.Background())

		return root.Execute()
	}

	return h
}

func testShopServer() *managementv3.ResourceServer {
	return &managementv3.ResourceServer{
		ID:                          testPtr("rs_shop"),
		Identifier:                  testPtr(poppyTestAPI),
		UseForPersonalAgentProtocol: testPtr(true),
		Scopes: []*managementv3.ResourceServerScope{
			{Value: "orders:read", Description: testPtr("See your orders")},
			{Value: "checkout", Description: testPtr("Place orders")},
		},
	}
}

func testOtherServer() *managementv3.ResourceServer {
	return &managementv3.ResourceServer{
		ID:         testPtr("rs_other"),
		Identifier: testPtr(poppyTestOtherAPI),
		Scopes:     []*managementv3.ResourceServerScope{{Value: "refund", Description: testPtr("Refund orders")}},
	}
}

func testAgentApp() *managementv3.Client {
	return &managementv3.Client{
		ClientID:               testPtr(poppyTestClientID),
		Name:                   testPtr("Meta Muse"),
		ExternalClientID:       testPtr(poppyTestAgentURL),
		ExternalMetadataType:   testPtr(managementv3.ClientExternalMetadataTypeEnumCimd),
		ExternalMetadataStatus: testPtr(managementv3.ClientExternalMetadataStatusEnumAvailable),
		AgentID:                testPtr(poppyTestAgentID),
	}
}

func testGrant(id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	return &managementv3.ClientGrantResponseContent{ID: testPtr(id), ClientID: testPtr(poppyTestClientID), Audience: testPtr(audience), Scope: scopes}
}

// testUserGrant is a grant for users of an app, the kind the APIs column counts.
func testUserGrant(clientID, id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	grant := testGrant(id, audience, scopes...)
	grant.ClientID = testPtr(clientID)
	grant.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumUser)

	return grant
}

func validPreview() *managementv3.PreviewCimdMetadataResponseContent {
	return &managementv3.PreviewCimdMetadataResponseContent{
		Validation:   &managementv3.CimdValidationResult{Valid: true},
		MappedFields: &managementv3.CimdMappedClientFields{Name: testPtr("Meta Muse")},
	}
}

// expectRemovalLookup sets the calls that plan the removal of the test agent.
// The resource servers are not read.
func (h *agentsHarness) expectRemovalLookup(grants ...*managementv3.ClientGrantResponseContent) {
	h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
	h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse", ExternalAgentID: testPtr(poppyTestAgentURL)}, nil)
	h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(grants...), nil)
}

// expectPoppyRemovalLookup sets the calls that plan the removal of the test
// agent limited to the Poppy APIs, which reads the resource servers once.
func (h *agentsHarness) expectPoppyRemovalLookup(grants ...*managementv3.ClientGrantResponseContent) {
	h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer(), testOtherServer()), nil)
	h.expectRemovalLookup(grants...)
}

func TestAgentsRegisterErrors(t *testing.T) {
	t.Run("the URL must be https", func(t *testing.T) {
		h := newAgentsHarness(t)

		err := h.root("register", "http://muse.example/agent.json")

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "invalid_agent_url", reason)
		assert.Contains(t, err.Error(), "must start with https://")
	})

	t.Run("an invalid document lists its violations", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(&managementv3.PreviewCimdMetadataResponseContent{
			Validation: &managementv3.CimdValidationResult{Violations: []string{"redirect_uris is empty"}},
		}, nil)

		err := h.root("register", poppyTestAgentURL, "--poppy", "--config", filepath.Join(t.TempDir(), "none.json"))

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "agent_invalid", reason)
		assert.Equal(t, []any{"redirect_uris is empty"}, details["violations"])
		assert.Equal(t, "the agent metadata document at "+poppyTestAgentURL+" is not valid", err.Error())

		var hinted apiResponseHintError
		require.ErrorAs(t, err, &hinted)
		assert.Equal(t, "  redirect_uris is empty", hinted.hint)
	})

	for _, test := range []struct {
		name       string
		violations []string
		wantReason string
	}{
		{name: "retrieval error", violations: []string{"Unexpected error during retrieval for " + poppyTestAgentURL}, wantReason: "agent_unreachable"},
		{name: "retrieval status", violations: []string{"Retrieval failed for " + poppyTestAgentURL + " with status 404"}, wantReason: "agent_unreachable"},
		{name: "retrieval and another violation", violations: []string{"Retrieval failed for " + poppyTestAgentURL + " with status 404", "redirect_uris is empty"}, wantReason: "agent_invalid"},
	} {
		t.Run("an unreachable document: "+test.name, func(t *testing.T) {
			h := newAgentsHarness(t)
			h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(&managementv3.PreviewCimdMetadataResponseContent{
				Validation: &managementv3.CimdValidationResult{Violations: test.violations},
			}, nil)

			err := h.root("register", poppyTestAgentURL, "--poppy", "--config", filepath.Join(t.TempDir(), "none.json"))

			reason, details := decodeCLIError(t, err)
			assert.Equal(t, test.wantReason, reason)
			assert.Len(t, details["violations"], len(test.violations))
			assert.Equal(t, "the agent metadata document at "+poppyTestAgentURL+" is not valid", err.Error())
			assert.NotContains(t, err.Error(), "\n")
		})
	}

	t.Run("scopes are required without a terminal", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)

		err := h.root("register", poppyTestAgentURL, "--poppy", "--config", filepath.Join(t.TempDir(), "none.json"))

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "scopes_required", reason)
		assert.Contains(t, err.Error(), "pass --scopes or --all-scopes")
		assert.Contains(t, err.Error(), poppyTestAPI+": orders:read, checkout")

		offers, ok := details["apis"].([]any)
		require.True(t, ok)
		require.Len(t, offers, 1)
		assert.Equal(t, poppyTestAPI, offers[0].(map[string]any)["audience"])
	})

	t.Run("an unknown scope", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)

		err := h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "nope", "--config", filepath.Join(t.TempDir(), "none.json"))

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "unknown_scope", reason)
	})

	t.Run("a registration with named scopes", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil)
		h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.CreateClientGrantResponseContent{ID: testPtr("cgr_new")}, nil)

		err := h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--json", "--config", filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, poppyTestClientID, got["client_id"])
		assert.Equal(t, poppyTestAgentID, got["agent_id"])
		assert.NotContains(t, got, "config_updated", "register changes the tenant only")
		assert.Empty(t, h.messages.String(), "nothing but the JSON")
	})

	t.Run("the output flags leave nothing on either stream but the JSON", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			h := newAgentsHarness(t)
			h.allowRegistration()

			require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", flag, "--config", filepath.Join(t.TempDir(), "none.json")), flag)

			assert.Empty(t, h.messages.String(), flag)
			assert.True(t, json.Valid(h.out.Bytes()), "%s: %s", flag, h.out.String())
		}
	})

	t.Run("the text lists the metadata, what was done and what to do next", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.appRequiresDPoP = true
		h.allowRegistration()
		h.clients.EXPECT().Update(gomock.Any(), poppyTestClientID, gomock.Any()).Return(&managementv3.UpdateClientResponseContent{}, nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--require-dpop", "--config", filepath.Join(t.TempDir(), "none.json")))

		out := h.out.String()
		assert.True(t, strings.HasPrefix(out, "Agent metadata  "+poppyTestAgentURL+"\n"), out)
		assert.Contains(t, out, "    name           Meta Muse\n")
		assert.Contains(t, out, "    keys           0 ()\n")
		assert.Contains(t, out, "  ok Agent app     created  "+poppyTestClientID+"\n")
		assert.Contains(t, out, "  ok DPoP          required\n")
		assert.Contains(t, out, "  ok Agent record  created  "+poppyTestAgentID+" (in tokens as "+poppyTestAgentURL+")\n")
		assert.Contains(t, out, "  ok Grant         "+poppyTestAPI+": orders:read\n")
		assert.True(t, strings.HasSuffix(out, "\nNext\n  auth0 agents show "+poppyTestAgentID+"   to check the agent.\n"), out)
		assert.Empty(t, h.messages.String())
	})

	t.Run("a grant the app already has is said so", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)

		existing := testGrant("cgr_old", poppyTestAPI, "orders:read")
		existing.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumUser)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(existing), nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--config", filepath.Join(t.TempDir(), "none.json")))

		assert.Contains(t, h.out.String(), "Grant         "+poppyTestAPI+": orders:read  (already granted)\n")
		assert.NotContains(t, h.out.String(), "FIX")
	})

	t.Run("a grant with other scopes than the ones requested says what differs and how to set them", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)

		existing := testGrant("cgr_old", poppyTestAPI, "orders:read")
		existing.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumUser)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(existing), nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read,checkout", "--config", filepath.Join(t.TempDir(), "none.json")))

		assert.Contains(t, h.out.String(), "(already granted)\n")
		assert.Contains(t, h.out.String(), "!  the existing grant for "+poppyTestAPI+" was left as it is and lacks checkout\n")
		assert.Contains(t, h.out.String(), "FIX  auth0 client-grants update cgr_old --scopes orders:read,checkout\n")
	})
}

func TestAgentsRegisterPlanText(t *testing.T) {
	h := newAgentsHarness(t)
	h.appRequiresDPoP = true
	h.allowRegistration()
	h.clients.EXPECT().Update(gomock.Any(), poppyTestClientID, gomock.Any()).Return(&managementv3.UpdateClientResponseContent{}, nil)

	run := h.runPromptMode(t, modeTerminal, []string{"register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--require-dpop", "--config", filepath.Join(t.TempDir(), "none.json")})

	require.NoError(t, run.err)
	assert.Equal(t, []string{"confirm"}, run.asked)
	assert.Contains(t, run.out, "\nThis will\n")
	assert.Contains(t, run.out, "  +  Agent app     create, for "+poppyTestAgentURL+"\n")
	assert.Contains(t, run.out, "  +  DPoP          required on the agent app\n")
	assert.Contains(t, run.out, `  +  Agent record  "Meta Muse"`+"\n")
	assert.Contains(t, run.out, "  +  Grant         "+poppyTestAPI+": orders:read  (skipped if the agent app already has a grant)\n")
	assert.Equal(t, "\nRegister the agent\nThis changes your Auth0 tenant: it creates or updates the agent app and grants it your APIs.\n", stripEscapes(run.errOut), "the question block goes to stderr")
}

func TestAgentsRegisterAPIs(t *testing.T) {
	out := func(t *testing.T) string { return filepath.Join(t.TempDir(), "none.json") }

	t.Run("any API can be named", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		require.NoError(t, h.root("register", poppyTestAgentURL, "--api", poppyTestOtherAPI, "--scopes", "refund", "--config", out(t)))

		assert.Equal(t, []string{poppyTestOtherAPI + " refund"}, *created)
		assert.Contains(t, h.out.String(), "Grant         "+poppyTestOtherAPI+": refund\n")
	})

	t.Run("an API by id, and several at once", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		require.NoError(t, h.root("register", poppyTestAgentURL, "--api", "rs_other", "--api", poppyTestAPI, "--all-scopes", "--config", out(t)))

		assert.Equal(t, []string{poppyTestOtherAPI + " refund", poppyTestAPI + " orders:read,checkout"}, *created)
	})

	t.Run("an API that does not exist", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer())

		err := h.root("register", poppyTestAgentURL, "--api", "https://nope.example", "--all-scopes", "--config", out(t))

		assert.EqualError(t, err, `"https://nope.example" is not an API of this tenant`)
		assert.Empty(t, *created)
	})

	t.Run("--poppy grants every Poppy API and no other", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testOtherServer(), testShopServer())

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--all-scopes", "--config", out(t)))

		assert.Equal(t, []string{poppyTestAPI + " orders:read,checkout"}, *created)
	})

	t.Run("--poppy with a Poppy API named", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testOtherServer(), testShopServer())

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--api", "rs_shop", "--scopes", "checkout", "--config", out(t)))

		assert.Equal(t, []string{poppyTestAPI + " checkout"}, *created)
	})

	t.Run("--poppy with an API that is not a Poppy API", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testOtherServer(), testShopServer())

		err := h.root("register", poppyTestAgentURL, "--poppy", "--api", poppyTestOtherAPI, "--scopes", "refund", "--config", out(t))

		assert.EqualError(t, err, `"`+poppyTestOtherAPI+`" is not a Poppy API`)
		assert.Empty(t, *created)
	})

	t.Run("--poppy on a tenant with no Poppy API", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testOtherServer())

		err := h.root("register", poppyTestAgentURL, "--poppy", "--all-scopes", "--config", out(t))

		assert.ErrorIs(t, err, poppyagents.ErrNoPoppyAPI)
		assert.ErrorContains(t, err, "Run: auth0 poppy setup")
		assert.Empty(t, *created)
	})

	t.Run("without a terminal and without --api or --poppy nothing is changed, and the APIs are listed in the details", func(t *testing.T) {
		modes := map[string]poppyPromptMode{
			"terminal with --no-input": modeTerminalNoInput,
			"piped":                    modePiped,
			"piped with --json":        modePipedJSON,
			"agent mode":               modeAgent,
		}

		for name, mode := range modes {
			h := newAgentsHarness(t)
			h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil).AnyTimes()
			h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer(), testOtherServer()), nil).Times(1)

			run := h.runPromptMode(t, mode, []string{"register", poppyTestAgentURL, "--all-scopes", "--config", out(t)})

			assert.Equal(t, "apis_required", errorReason(run.err), name)
			assert.ErrorContains(t, run.err, "pass --api to choose the APIs to grant, or --poppy", name)
			assert.Empty(t, run.out, name)

			var details struct {
				APIs []struct {
					Audience string `json:"audience"`
					Name     string `json:"name"`
					Poppy    bool   `json:"poppy"`
					Scopes   []struct {
						Name        string `json:"name"`
						Description string `json:"description"`
					} `json:"scopes"`
				} `json:"apis"`
			}
			require.NoError(t, json.Unmarshal(errorDetails(run.err), &details), name)
			require.Len(t, details.APIs, 2, name)
			assert.Equal(t, poppyTestAPI, details.APIs[0].Audience, name)
			assert.True(t, details.APIs[0].Poppy, name)
			assert.False(t, details.APIs[1].Poppy, name)
			require.Len(t, details.APIs[0].Scopes, 2, name)
			assert.Equal(t, "orders:read", details.APIs[0].Scopes[0].Name, name)
			assert.Equal(t, "See your orders", details.APIs[0].Scopes[0].Description, name)
		}
	})

	t.Run("the Management API is not offered in the details", func(t *testing.T) {
		management := &managementv3.ResourceServer{ID: testPtr("rs_mgmt"), Identifier: testPtr(poppyTestManagementAPI), Name: testPtr("Auth0 Management API")}

		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil).AnyTimes()
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(management, testShopServer(), testOtherServer()), nil).Times(1)

		run := h.runPromptMode(t, modePiped, []string{"register", poppyTestAgentURL, "--all-scopes", "--config", out(t)})

		assert.Equal(t, "apis_required", errorReason(run.err))

		var details struct {
			APIs []struct {
				Audience string `json:"audience"`
			} `json:"apis"`
		}
		require.NoError(t, json.Unmarshal(errorDetails(run.err), &details))

		var audiences []string
		for _, api := range details.APIs {
			audiences = append(audiences, api.Audience)
		}

		assert.Equal(t, []string{poppyTestAPI, poppyTestOtherAPI}, audiences)
	})

	t.Run("without the scope to list the APIs, apis_required has no details", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil).AnyTimes()
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("forbidden")).Times(1)

		run := h.runPromptMode(t, modePiped, []string{"register", poppyTestAgentURL, "--all-scopes", "--config", out(t)})

		assert.Equal(t, "apis_required", errorReason(run.err))
		assert.Nil(t, errorDetails(run.err))
	})
}

// runTerminal runs an agents command as in a terminal where the APIs are chosen
// by index in chooseAPIs, and the confirmations are answered by confirm. It
// returns the questions asked.
func (h *agentsHarness) runTerminal(t *testing.T, chooseAPIs []int, confirm func(message string) bool, args ...string) (asked []string, err error) {
	t.Helper()

	installPoppyAsker(t, modeTerminal)

	promptAsk.chooseMany = func(message string, _ []string, _ []int) ([]int, error) {
		asked = append(asked, "choose "+message)

		return chooseAPIs, nil
	}
	promptAsk.confirm = func(message string, _ bool) (bool, error) {
		asked = append(asked, "confirm "+message)

		return confirm(message), nil
	}

	err = h.root(args...)

	return asked, err
}

func TestAgentsRegisterAsksForTheAPIs(t *testing.T) {
	always := func(string) bool { return true }

	t.Run("every API of the tenant is offered, the Poppy ones marked", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testOtherServer(), testShopServer())

		var rows []string

		installPoppyAsker(t, modeTerminal)
		promptAsk.chooseMany = func(message string, options []string, defaults []int) ([]int, error) {
			assert.Equal(t, "APIs to grant", message)
			assert.Empty(t, defaults, "nothing selected to begin with")
			rows = options

			return []int{0}, nil
		}

		require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "refund", "--config", filepath.Join(t.TempDir(), "none.json")))

		assert.Equal(t, []string{poppyTestOtherAPI, poppyTestAPI + "  (Poppy)"}, rows)
		assert.Equal(t, []string{poppyTestOtherAPI + " refund"}, *created)
		assert.True(t, strings.HasPrefix(h.out.String(), "Agent metadata  "+poppyTestAgentURL+"\n"), "the metadata comes before the questions")
	})

	t.Run("the Management API is not offered in the picker", func(t *testing.T) {
		management := &managementv3.ResourceServer{ID: testPtr("rs_mgmt"), Identifier: testPtr(poppyTestManagementAPI), Name: testPtr("Auth0 Management API")}

		h := newAgentsHarness(t)
		h.allowRegistrationOn(management, testShopServer())

		var rows []string

		installPoppyAsker(t, modeTerminal)
		promptAsk.chooseMany = func(_ string, options []string, _ []int) ([]int, error) {
			rows = options

			return []int{0}, nil
		}

		require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "orders:read", "--config", filepath.Join(t.TempDir(), "none.json")))

		assert.Equal(t, []string{poppyTestAPI + "  (Poppy)"}, rows)
	})

	t.Run("no API chosen needs a confirmation", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer())

		asked, err := h.runTerminal(t, nil, always, "register", poppyTestAgentURL, "--config", filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)
		assert.Equal(t, []string{
			"choose APIs to grant",
			"confirm Register the agent without access to any API?",
			"confirm Register this agent?",
		}, asked)
		assert.Empty(t, *created)
		assert.Contains(t, h.out.String(), "Agent record  created")
	})

	t.Run("no API chosen and not confirmed registers nothing", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil).AnyTimes()

		_, err := h.runTerminal(t, nil, func(string) bool { return false }, "register", poppyTestAgentURL, "--config", filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)
		assert.Contains(t, h.messages.String(), "Nothing was registered.")
	})

	t.Run("--yes confirms having no API", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer())

		asked, err := h.runTerminal(t, nil, always, "register", poppyTestAgentURL, "--force", "--config", filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)
		assert.Equal(t, []string{"choose APIs to grant"}, asked)
		assert.Empty(t, *created)
	})

	t.Run("a tenant with no API goes straight to the confirmation", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.allowRegistrationOn()

		asked, err := h.runTerminal(t, nil, always, "register", poppyTestAgentURL, "--config", filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)
		assert.Equal(t, []string{
			"confirm Register the agent without access to any API?",
			"confirm Register this agent?",
		}, asked)
	})
}

func TestAgentsUnregister(t *testing.T) {
	shop := testGrant("cgr_shop", poppyTestAPI, "orders:read")
	other := testGrant("cgr_other", poppyTestOtherAPI, "refund")

	t.Run("not an agent app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(&managementv3.Client{ClientID: testPtr("client_plain"), Name: testPtr("Plain"), AgentID: testPtr("agt_plain")}), nil)
		h.agents.EXPECT().Read(gomock.Any(), "agt_plain").Return(&managementv3.AgentResponseContent{AgentID: "agt_plain", Name: "Plain"}, nil)

		err := h.root("unregister", "agt_plain", "--force")

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "not_an_agent", reason)
		assert.Contains(t, err.Error(), "isn't an agent")
		assert.Equal(t, "auth0 agents delete agt_plain", details["next"])
	})

	t.Run("limited to Poppy APIs, no Poppy grant", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(other)

		err := h.root("unregister", poppyTestAgentURL, "--poppy", "--force")

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "no_poppy_grant", reason)
		assert.Contains(t, err.Error(), "has no grant on a Poppy API, so nothing was removed")
		assert.Equal(t, "auth0 agents unregister "+poppyTestAgentURL, details["next"])
	})

	t.Run("the grants flag of old is gone", func(t *testing.T) {
		h := newAgentsHarness(t)

		err := h.root("unregister", poppyTestAgentURL, "--remove-all-grants")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown flag: --remove-all-grants")
	})

	t.Run("confirmation is required without a terminal", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectRemovalLookup(shop)

		err := h.root("unregister", poppyTestAgentURL)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "confirmation_required", reason)
		assert.Equal(t, true, details["dry_run"])
		assert.Equal(t, poppyTestAgentURL, details["url"])
		assert.Contains(t, h.out.String(), "This will remove:")
		assert.Contains(t, h.out.String(), "client grant cgr_shop ("+poppyTestAPI+")")
		assert.Empty(t, h.messages.String())
	})

	t.Run("a dry run changes nothing and plans every grant, the record and the app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectRemovalLookup(shop, other)

		h.json = true
		h.renderer.Format = display.OutputFormatJSON

		err := h.root("unregister", poppyTestAgentURL, "--dry-run")

		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, true, got["dry_run"])
		assert.Equal(t, []any{
			map[string]any{"kind": "client_grant", "id": "cgr_shop", "audience": poppyTestAPI},
			map[string]any{"kind": "client_grant", "id": "cgr_other", "audience": poppyTestOtherAPI},
			map[string]any{"kind": "agent_record", "id": poppyTestAgentID},
			map[string]any{"kind": "agent_app", "id": poppyTestClientID},
		}, got["removed"])
		assert.Empty(t, got["kept"])
	})

	t.Run("the output flags print the result object", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			h := newAgentsHarness(t)
			h.expectPoppyRemovalLookup(shop, other)

			err := h.root("unregister", poppyTestAgentURL, "--poppy", "--dry-run", flag)

			require.NoError(t, err, flag)

			var got map[string]any
			require.NoError(t, json.Unmarshal(h.out.Bytes(), &got), flag)
			assert.Equal(t, true, got["dry_run"])
			assert.Contains(t, got, "removed")
			assert.Contains(t, got, "kept")
			assert.Contains(t, got, "remaining_grants")
		}
	})

	t.Run("the output flags exclude each other", func(t *testing.T) {
		h := newAgentsHarness(t)

		err := h.root("unregister", poppyTestAgentURL, "--json", "--json-compact")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "none of the others can be")
	})

	t.Run("a forced removal takes every grant, the record and the app without reading the resource servers", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectRemovalLookup(shop, other)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_other").Return(nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
		h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--force"))

		assert.Equal(t, "Removed:\n"+
			"  ok client grant cgr_shop ("+poppyTestAPI+")\n"+
			"  ok client grant cgr_other ("+poppyTestOtherAPI+")\n"+
			"  ok agent record "+poppyTestAgentID+"\n"+
			"  ok agent app "+poppyTestClientID+"\n"+
			"\n"+
			"  !  The refresh tokens of the app were revoked with it. Access tokens already issued stay valid until they expire.\n"+
			"  !  Registering the agent again creates a new app, so its users must sign in again.\n",
			h.out.String())
		assert.Empty(t, h.messages.String())
	})

	t.Run("an agent app with no grant is removed", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectRemovalLookup()
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
		h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--force"))

		assert.Contains(t, h.out.String(), "  ok agent record "+poppyTestAgentID+"\n  ok agent app "+poppyTestClientID+"\n")
	})

	t.Run("limited to Poppy APIs, the other grants stay and so do the record and the app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(shop, other)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)

		h.json = true
		h.renderer.Format = display.OutputFormatJSON

		err := h.root("unregister", poppyTestAgentURL, "--poppy", "--force")

		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, false, got["dry_run"])
		assert.Equal(t, []any{map[string]any{"kind": "client_grant", "id": "cgr_shop", "audience": poppyTestAPI}}, got["removed"])
		assert.Equal(t, []any{
			map[string]any{"kind": "agent_record", "id": poppyTestAgentID},
			map[string]any{"kind": "agent_app", "id": poppyTestClientID},
		}, got["kept"])
		assert.Len(t, got["remaining_grants"], 1)
	})

	t.Run("limited to Poppy APIs with no other grant, the record and the app go too", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(shop)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
		h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--poppy", "--force"))

		assert.Contains(t, h.out.String(), "  ok agent record "+poppyTestAgentID+"\n  ok agent app "+poppyTestClientID+"\n")
	})

	t.Run("limited to Poppy APIs, a dry run says what would be removed and kept", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(shop, other)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--poppy", "--dry-run"))

		assert.Equal(t, "This would remove:\n"+
			"  client grant cgr_shop ("+poppyTestAPI+")\n"+
			"\n"+
			"This would keep:\n"+
			"  agent record "+poppyTestAgentID+"\n"+
			"  agent app "+poppyTestClientID+"\n"+
			"\n"+
			"Other grants of the agent app stay. Remove them with:\n"+
			"  auth0 client-grants delete cgr_other   ("+poppyTestOtherAPI+")\n",
			h.out.String())
		assert.Empty(t, h.messages.String())
	})

	t.Run("a removal that keeps the app says what was kept", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(shop, other)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--poppy", "--force"))

		out := h.out.String()
		assert.Contains(t, out, "Removed:\n  ok client grant cgr_shop")
		assert.Contains(t, out, "Kept:\n  -  agent record "+poppyTestAgentID+"\n  -  agent app "+poppyTestClientID+"\n")
		assert.Contains(t, out, "auth0 client-grants delete cgr_other   ("+poppyTestOtherAPI+")")
		assert.Contains(t, out, "unregister it without --poppy")
		assert.NotContains(t, out, "Registering the agent again", "the app stays, so nothing is created again")
		assert.Empty(t, h.messages.String())
	})

	t.Run("an app that is gone is said so", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL))

		assert.Contains(t, h.out.String(), "!  No agent app was found for "+poppyTestAgentURL+". It may already be unregistered.")
		assert.Empty(t, h.messages.String())
	})

	t.Run("the output flags leave nothing on either stream but the JSON", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			gone := newAgentsHarness(t)
			gone.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
			gone.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)
			require.NoError(t, gone.root("unregister", poppyTestAgentURL, flag), flag)
			assert.Empty(t, gone.messages.String(), flag)
			assert.True(t, json.Valid(gone.out.Bytes()), "%s: %s", flag, gone.out.String())

			refused := newAgentsHarness(t)
			refused.expectRemovalLookup(shop)
			err := refused.root("unregister", poppyTestAgentURL, flag)
			assert.Equal(t, "confirmation_required", errorReason(err), flag)
			assert.Empty(t, refused.messages.String(), flag)
			assert.Empty(t, refused.out.String(), flag)

			forced := newAgentsHarness(t)
			forced.expectRemovalLookup(shop)
			forced.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
			forced.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
			forced.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)
			require.NoError(t, forced.root("unregister", poppyTestAgentURL, "--force", flag), flag)
			assert.Empty(t, forced.messages.String(), flag)
			assert.True(t, json.Valid(forced.out.Bytes()), "%s: %s", flag, forced.out.String())
		}
	})

	t.Run("a partial failure keeps its details", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectRemovalLookup(shop)
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(errors.New("boom"))

		err := h.root("unregister", poppyTestAgentURL, "--force")

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "partial_failure", reason)
		assert.Contains(t, err.Error(), "stopped partway")
		assert.Equal(t, []any{"delete client grant cgr_shop (" + poppyTestAPI + ")"}, details["applied"])
		assert.Equal(t, "delete agent record "+poppyTestAgentID, details["failed"])
		assert.Equal(t, []any{"delete agent app " + poppyTestClientID}, details["remaining"])
		assert.Equal(t, "auth0 agents unregister "+poppyTestAgentURL+" --force", details["next"])
	})

	t.Run("without an argument and without a terminal, the agent is required", func(t *testing.T) {
		h := newAgentsHarness(t)

		err := h.root("unregister", "--force")

		assert.Equal(t, "missing_required_args", errorReason(err))
	})

	t.Run("limited to Poppy APIs, a removal that stopped after the grants goes on with the record and the app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup()
		gomock.InOrder(
			h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil),
			h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil),
		)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--poppy", "--force"))

		assert.Contains(t, h.out.String(), "  ok agent record "+poppyTestAgentID+"\n  ok agent app "+poppyTestClientID+"\n")
	})

	t.Run("limited to Poppy APIs, other grants and no Poppy grant delete nothing", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectPoppyRemovalLookup(other)

		// No Delete is expected: any call fails the test.
		err := h.root("unregister", poppyTestAgentURL, "--poppy", "--force")

		assert.Equal(t, "no_poppy_grant", errorReason(err))
	})

	t.Run("an agent record without an app is removed by URL", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		// Only the record: there is no app and no grant to delete.
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentURL, "--force"))

		assert.Contains(t, h.out.String(), "  ok agent record "+poppyTestAgentID+"\n")
		assert.NotContains(t, h.out.String(), "No agent app was found")
	})

	t.Run("an agent record without an app is removed by agent ID", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)

		require.NoError(t, h.root("unregister", poppyTestAgentID, "--force"))

		assert.Contains(t, h.out.String(), "  ok agent record "+poppyTestAgentID+"\n")
	})

	t.Run("an agent that is gone by ID is said so", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(nil, core.NewAPIError(http.StatusNotFound, nil, errors.New("gone")))

		require.NoError(t, h.root("unregister", poppyTestAgentID))

		assert.Contains(t, h.out.String(), "!  No agent app was found for "+poppyTestAgentID)
	})
}

func TestUnregisterPickerOptions(t *testing.T) {
	muse := testAgentApp()
	shopApp := testAgentApp()
	shopApp.ClientID, shopApp.AgentID, shopApp.ExternalClientID, shopApp.Name = testPtr("client_shop"), testPtr("agt_shop"), testPtr("https://shop.example/agent.json"), testPtr("Shop")
	plain := &managementv3.Client{ClientID: testPtr("client_plain"), Name: testPtr("Plain"), AgentID: testPtr("agt_plain")}

	t.Run("the agents that have an agent app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(muse, plain, shopApp), nil)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)

		opts, err := h.cli.unregisterPickerOptions(false)(t.Context())

		require.NoError(t, err)
		require.Len(t, opts, 2)
		assert.Equal(t, poppyTestAgentURL, opts[0].value)
		assert.Contains(t, opts[0].label, "Meta Muse")
		assert.Equal(t, "https://shop.example/agent.json", opts[1].value)
	})

	t.Run("with --poppy, those with grants but none on a Poppy API are left out", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer(), testOtherServer()), nil).Times(1)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(muse, shopApp), nil)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)
		// One call for the grants of every agent.
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(
			testUserGrant(poppyTestClientID, "cgr_other", poppyTestOtherAPI, "refund"),
		), nil).Times(1)

		opts, err := h.cli.unregisterPickerOptions(true)(t.Context())

		require.NoError(t, err)
		require.Len(t, opts, 1)
		assert.Equal(t, "https://shop.example/agent.json", opts[0].value)
	})

	t.Run("no agent to pick", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(plain), nil)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)

		_, err := h.cli.unregisterPickerOptions(false)(t.Context())

		assert.ErrorContains(t, err, "no agents to unregister")
	})
}

// runPromptMode runs an agents command of the harness in a mode and returns what it did.
func (h *agentsHarness) runPromptMode(t *testing.T, mode poppyPromptMode, args []string) poppyPromptRun {
	t.Helper()

	asked := installPoppyAsker(t, mode)
	h.agentMode = mode.agentMode

	err := h.root(append(args, mode.flags...)...)

	return poppyPromptRun{asked: *asked, out: h.out.String(), errOut: h.messages.String(), err: err}
}

// registeredApp is the app that registration reads back.
func (h *agentsHarness) registeredApp() *managementv3.Client {
	app := testAgentApp()
	app.RequireProofOfPossession = testPtr(h.appRequiresDPoP)

	return app
}

// allowRegistration accepts every call that registering the test agent makes.
func (h *agentsHarness) allowRegistration() {
	h.allowRegistrationOn(testShopServer())
}

// allowRegistrationOn accepts every call that registering the test agent makes,
// on a tenant with these APIs. It returns the audiences of the grants created.
func (h *agentsHarness) allowRegistrationOn(servers ...*managementv3.ResourceServer) *[]string {
	var created []string

	h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil).AnyTimes()
	h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(servers...), nil).AnyTimes()
	h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil).AnyTimes()
	h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil).AnyTimes()
	h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(h.registeredApp()), nil).AnyTimes()
	h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil).AnyTimes()
	h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
			created = append(created, request.Audience+" "+strings.Join(request.Scope, ","))

			return &managementv3.CreateClientGrantResponseContent{ID: testPtr("cgr_new")}, nil
		}).AnyTimes()

	return &created
}

func TestAgentsRegisterPromptRule(t *testing.T) {
	asks := []string{"confirm"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Agent app"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: "Agent metadata", json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "Agent app"},
		{name: "piped", mode: modePiped, text: "Agent app"},
		{name: "piped with --json", mode: modePipedJSON, json: true},
		{name: "agent mode", mode: modeAgent, json: true},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, message: "Agent metadata", json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newAgentsHarness(t)
			h.allowRegistration()

			run := h.runPromptMode(t, tc.mode, []string{"register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read", "--config", filepath.Join(t.TempDir(), "none.json")})

			assertPoppyPromptRun(t, tc, run)
		})
	}
}

func TestAgentsRegisterPromptRuleScopes(t *testing.T) {
	const missing = "pass --scopes or --all-scopes"

	// The question a terminal gets here is a survey prompt that no test can answer,
	// so the modes that ask are left out.
	tests := []poppyPromptRow{
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "Agent metadata", reason: "scopes_required", errText: missing},
		{name: "piped", mode: modePiped, text: "Agent metadata", reason: "scopes_required", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "scopes_required", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "scopes_required", errText: missing},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newAgentsHarness(t)
			h.allowRegistration()

			run := h.runPromptMode(t, tc.mode, []string{"register", poppyTestAgentURL, "--poppy", "--config", filepath.Join(t.TempDir(), "none.json")})

			assertPoppyPromptRun(t, tc, run)
		})
	}
}

func TestAgentsUnregisterPromptRule(t *testing.T) {
	const refused = "--force"

	asks := []string{"confirm"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Removed:"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: "This will remove:", json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "This will remove:", reason: "confirmation_required", errText: refused},
		{name: "piped", mode: modePiped, text: "This will remove:", reason: "confirmation_required", errText: refused},
		{name: "piped with --json", mode: modePipedJSON, reason: "confirmation_required", errText: refused},
		{name: "agent mode", mode: modeAgent, reason: "confirmation_required", errText: refused},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, message: "This will remove:", json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newAgentsHarness(t)
			h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
			h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil).AnyTimes()
			h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil).AnyTimes()
			h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil).AnyTimes()

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, []string{"unregister", poppyTestAgentURL}))
		})
	}
}

func TestPromptPoppyGrants(t *testing.T) {
	apis := []coreagents.API{{
		Identifier: poppyTestAPI,
		Scopes: []coreagents.Scope{
			{Name: "orders:read", Description: "See your orders"},
			{Name: "checkout", Description: "Place orders"},
			{Name: "profile", Description: "See your profile"},
		},
	}}

	cfg := &config.Config{Scopes: map[string]config.Scope{
		"orders:read": {Hints: &config.Hints{Changes: "none"}},
		"checkout":    {Hints: &config.Hints{Changes: "irreversible", PersonalData: true}},
	}}

	tests := []struct {
		name         string
		cfg          *config.Config
		picked       []int
		wantDefaults []int
		want         []coreagents.Grant
		wantReason   string
	}{
		{
			name:         "scopes that change nothing are selected to begin with",
			cfg:          cfg,
			picked:       []int{0, 2},
			wantDefaults: []int{0},
			want:         []coreagents.Grant{{Audience: poppyTestAPI, Scopes: []string{"orders:read", "profile"}}},
		},
		{
			name:       "no hints, nothing selected to begin with",
			picked:     []int{1},
			want:       []coreagents.Grant{{Audience: poppyTestAPI, Scopes: []string{"checkout"}}},
			wantReason: "",
		},
		{name: "nothing chosen is an error", cfg: cfg, wantDefaults: []int{0}, wantReason: "scopes_required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				defaults []int
				rows     []string
			)

			screen, out := newTestPromptScreen(t, promptAsker{
				chooseMany: func(_ string, options []string, d []int) ([]int, error) {
					defaults, rows = d, options

					return test.picked, nil
				},
			})

			grants, err := promptAgentGrants(screen, apis, test.cfg)

			assert.Equal(t, test.wantDefaults, defaults)
			assert.Len(t, rows, 3)
			assert.Contains(t, out.String(), "Scopes to grant on "+poppyTestAPI)

			if test.wantReason != "" {
				assert.Equal(t, test.wantReason, errorReason(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, grants)
		})
	}
}

func TestAskUnregister(t *testing.T) {
	tests := []struct {
		name      string
		deleteApp bool
		confirm   bool
		wantText  string
	}{
		{name: "the app goes too, confirmed", deleteApp: true, confirm: true, wantText: "its refresh tokens"},
		{name: "the app goes too, declined", deleteApp: true},
		{name: "the app stays, confirmed", confirm: true, wantText: "because the app has other grants"},
		{name: "the app stays, declined"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			screen, out := newTestPromptScreen(t, promptAsker{
				confirm: func(string, bool) (bool, error) { return test.confirm, nil },
			})

			proceed, err := askUnregister(screen, test.deleteApp)

			require.NoError(t, err)
			assert.Equal(t, test.confirm, proceed)
			assert.Contains(t, out.String(), "Unregister the agent")

			if test.wantText != "" {
				assert.Contains(t, out.String(), test.wantText)
			}
		})
	}
}

func TestPoppyRef(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("no-input", false, "")

	t.Run("an argument that is given is used", func(t *testing.T) {
		installPoppyAsker(t, modePiped)

		ref, err := agentRef(cmd, nil, agentURLArgument, []string{"https://agent.example/agent.json"})

		require.NoError(t, err)
		assert.Equal(t, "https://agent.example/agent.json", ref)
	})

	t.Run("without a terminal it is required", func(t *testing.T) {
		installPoppyAsker(t, modePiped)

		_, err := agentRef(cmd, nil, agentURLArgument, nil)

		assert.Equal(t, "missing_required_args", errorReason(err))
		assert.ErrorContains(t, err, "Agent URL")
	})

	t.Run("a terminal asks for it", func(t *testing.T) {
		installPoppyAsker(t, modeTerminal)

		screen, out := newTestPromptScreen(t, promptAsker{
			text: func(label, _ string) (string, error) {
				assert.Equal(t, "URL", label)

				return "https://agent.example/agent.json", nil
			},
		})

		ref, err := agentRef(cmd, screen, agentURLArgument, nil)

		require.NoError(t, err)
		assert.Equal(t, "https://agent.example/agent.json", ref)
		assert.Contains(t, out.String(), "Agent URL\nURL of the agent's metadata document.\n")
	})
}

func TestAgentsRegisterFailures(t *testing.T) {
	apiShopOther := []string{"register", poppyTestAgentURL, "--api", poppyTestAPI, "--api", poppyTestOtherAPI}

	t.Run("a registration that stops partway is a classified partial failure", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, core.NewAPIError(http.StatusInternalServerError, nil, errors.New("boom")))

		err := h.root("register", poppyTestAgentURL, "--api", poppyTestAPI, "--scopes", "orders:read")

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "partial_failure", reason)
		assert.Equal(t, []any{"register agent app"}, details["applied"])
		assert.Equal(t, "create agent record", details["failed"])
		assert.Equal(t, []any{"read agent app", "grant " + poppyTestAPI}, details["remaining"])
		assert.Contains(t, details["next"], "auth0 agents register "+poppyTestAgentURL)
		assert.Contains(t, details["next"], "--scopes orders:read")
	})

	t.Run("an existing record that another app is linked to is a link mismatch", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, core.NewAPIError(http.StatusConflict, nil, errors.New("exists")))
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)

		linked := testAgentApp()
		linked.AgentID = testPtr("agt_someone_else")
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(linked), nil)

		// Nothing is linked or granted: no Update, no grant call.
		err := h.root("register", poppyTestAgentURL, "--api", poppyTestAPI, "--scopes", "orders:read")

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "agent_link_mismatch", reason)
		assert.Contains(t, details["next"], poppyTestClientID)
		assert.Contains(t, details["next"], poppyTestAgentID)
	})

	t.Run("an existing record the app is not linked to gets linked", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, core.NewAPIError(http.StatusConflict, nil, errors.New("exists")))
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)

		unlinked := testAgentApp()
		unlinked.AgentID = nil
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(unlinked), nil)
		h.clients.EXPECT().
			Update(gomock.Any(), poppyTestClientID, &managementv3.UpdateClientRequestContent{AgentID: testPtr(poppyTestAgentID)}).
			Return(&managementv3.UpdateClientResponseContent{}, nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil)
		h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.CreateClientGrantResponseContent{ID: testPtr("cgr_new")}, nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--api", poppyTestAPI, "--scopes", "orders:read"))
	})

	t.Run("several APIs each get the listed scopes they offer", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		require.NoError(t, h.root(append(apiShopOther, "--scopes", "orders:read,refund,orders:read")...))

		assert.ElementsMatch(t, []string{poppyTestAPI + " orders:read", poppyTestOtherAPI + " refund"}, *created)
	})

	t.Run("a scope can be pinned to one API", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		require.NoError(t, h.root(append(apiShopOther, "--scopes", poppyTestAPI+"=checkout,"+poppyTestOtherAPI+"=refund")...))

		assert.ElementsMatch(t, []string{poppyTestAPI + " checkout", poppyTestOtherAPI + " refund"}, *created)
	})

	t.Run("a scope no chosen API offers is unknown, and names the APIs", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		err := h.root(append(apiShopOther, "--scopes", "orders:read,nope")...)

		assert.Equal(t, "unknown_scope", errorReason(err))
		assert.ErrorContains(t, err, "nope")
		assert.ErrorContains(t, err, poppyTestAPI)
		assert.Empty(t, *created)
	})

	t.Run("an API left without a scope needs one, and the details list what it offers", func(t *testing.T) {
		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer(), testOtherServer())

		err := h.root(append(apiShopOther, "--scopes", "orders:read")...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "scopes_required", reason)
		assert.Empty(t, *created)

		offers, ok := details["apis"].([]any)
		require.True(t, ok)
		require.Len(t, offers, 1)
		assert.Equal(t, poppyTestOtherAPI, offers[0].(map[string]any)["audience"])
	})
}

func TestAgentsRegisterScopeHints(t *testing.T) {
	registerShop := func(h *agentsHarness, extra ...string) error {
		return h.root(append([]string{"register", poppyTestAgentURL, "--api", poppyTestAPI, "--scopes", "orders:read"}, extra...)...)
	}

	t.Run("a config file that is given but cannot be parsed is warned about and left out", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "poppy.json")
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

		h := newAgentsHarness(t)
		created := h.allowRegistrationOn(testShopServer())

		require.NoError(t, registerShop(h, "--config", path))

		assert.Contains(t, h.messages.String(), "The scope hints in "+path+" are not used")
		assert.Equal(t, []string{poppyTestAPI + " orders:read"}, *created)
	})

	t.Run("the default config file is not read without --poppy or --config", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "poppy.json"), []byte("{not json"), 0o600))
		t.Chdir(dir)

		h := newAgentsHarness(t)
		h.allowRegistration()

		require.NoError(t, registerShop(h))

		assert.NotContains(t, h.messages.String(), "scope hints")
	})

	t.Run("the default config file is read with --poppy", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "poppy.json"), []byte("{not json"), 0o600))
		t.Chdir(dir)

		h := newAgentsHarness(t)
		h.allowRegistration()

		require.NoError(t, h.root("register", poppyTestAgentURL, "--poppy", "--scopes", "orders:read"))

		assert.Contains(t, h.messages.String(), "The scope hints in poppy.json are not used")
	})

	t.Run("a config file that is not there is not an error", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.allowRegistration()

		require.NoError(t, registerShop(h, "--config", filepath.Join(t.TempDir(), "none.json")))

		assert.NotContains(t, h.messages.String(), "scope hints")
	})
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
)

const (
	poppyTestAgentURL = "https://muse.example/agent.json"
	poppyTestAPI      = "https://api.shop.example"
	poppyTestClientID = "client_muse"
	poppyTestAgentID  = "agt_muse"
)

type poppyAgentsHarness struct {
	clients     *mock.MockClientAPIV3
	agents      *mock.MockAgentAPIV3
	grants      *mock.MockClientGrantAPIV3
	servers     *mock.MockResourceServerAPIV3
	credentials *mock.MockClientCredentialAPIV3
	out         *bytes.Buffer
	messages    *bytes.Buffer
	json        bool
	agentMode   bool
	renderer    *display.Renderer
	root        func(args ...string) error
}

func newPoppyAgentsHarness(t *testing.T) *poppyAgentsHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &poppyAgentsHarness{
		clients:     mock.NewMockClientAPIV3(ctrl),
		agents:      mock.NewMockAgentAPIV3(ctrl),
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
			Agent:            h.agents,
			ClientGrant:      h.grants,
			ResourceServerV3: h.servers,
			ClientCredential: h.credentials,
		},
	}

	h.renderer = renderer

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
		root.SetArgs(append([]string{"poppy", "agents"}, args...))
		root.SetContext(context.Background())

		return root.Execute()
	}

	return h
}

func testPage[C comparable, T, R any](items ...T) *core.Page[C, T, R] {
	return &core.Page[C, T, R]{
		Results:      items,
		NextPageFunc: func(context.Context) (*core.Page[C, T, R], error) { return nil, core.ErrNoPages },
	}
}

func testServerPage(items ...*managementv3.ResourceServer) *auth0.ResourceServerPage {
	return testPage[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent](items...)
}

func testClientPage(items ...*managementv3.Client) *auth0.ClientListPage {
	return testPage[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent](items...)
}

func testGrantPage(items ...*managementv3.ClientGrantResponseContent) *auth0.ClientGrantPage {
	return testPage[*string, *managementv3.ClientGrantResponseContent, *managementv3.ListClientGrantPaginatedResponseContent](items...)
}

func testAgentPage(items ...*managementv3.AgentResponseContent) *auth0.AgentPage {
	return testPage[*string, *managementv3.AgentResponseContent, *managementv3.ListAgentsResponseContent](items...)
}

func testPtr[T any](value T) *T { return &value }

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
	return &managementv3.ClientGrantResponseContent{ID: testPtr(id), Audience: testPtr(audience), Scope: scopes}
}

func validPreview() *managementv3.PreviewCimdMetadataResponseContent {
	return &managementv3.PreviewCimdMetadataResponseContent{
		Validation:   &managementv3.CimdValidationResult{Valid: true},
		MappedFields: &managementv3.CimdMappedClientFields{Name: testPtr("Meta Muse")},
	}
}

// expectRemovalLookup sets the calls that plan the removal of the test agent.
func (h *poppyAgentsHarness) expectRemovalLookup(grants ...*managementv3.ClientGrantResponseContent) {
	h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
	h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse", ExternalAgentID: testPtr(poppyTestAgentURL)}, nil)
	h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
	h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(grants...), nil)
}

func decodeCLIError(t *testing.T, err error) (reason string, details map[string]any) {
	t.Helper()

	require.Error(t, err)
	reason = errorReason(err)

	if raw := errorDetails(err); raw != nil {
		require.NoError(t, json.Unmarshal(raw, &details))
	}

	return reason, details
}

func TestPoppyAgentsRegisterErrors(t *testing.T) {
	t.Run("the URL must be https", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)

		err := h.root("register", "http://muse.example/agent.json")

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "invalid_agent_url", reason)
		assert.Contains(t, err.Error(), "must start with https://")
	})

	t.Run("an invalid document lists its violations", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(&managementv3.PreviewCimdMetadataResponseContent{
			Validation: &managementv3.CimdValidationResult{Violations: []string{"redirect_uris is empty"}},
		}, nil)

		err := h.root("register", poppyTestAgentURL, "--out", filepath.Join(t.TempDir(), "none.json"))

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "agent_invalid", reason)
		assert.Equal(t, []any{"redirect_uris is empty"}, details["violations"])
	})

	t.Run("scopes are required without a terminal", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)

		err := h.root("register", poppyTestAgentURL, "--out", filepath.Join(t.TempDir(), "none.json"))

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
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)

		err := h.root("register", poppyTestAgentURL, "--scopes", "nope", "--out", filepath.Join(t.TempDir(), "none.json"))

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "unknown_scope", reason)
	})

	t.Run("a registration with named scopes", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil)
		h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.CreateClientGrantResponseContent{ID: testPtr("cgr_new")}, nil)

		err := h.root("register", poppyTestAgentURL, "--scopes", "orders:read", "--json", "--out", filepath.Join(t.TempDir(), "none.json"))

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
			h := newPoppyAgentsHarness(t)
			h.allowRegistration()

			require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "orders:read", flag, "--out", filepath.Join(t.TempDir(), "none.json")), flag)

			assert.Empty(t, h.messages.String(), flag)
			assert.True(t, json.Valid(h.out.Bytes()), "%s: %s", flag, h.out.String())
		}
	})

	t.Run("the text lists the metadata, what was done and what to do next", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.allowRegistration()
		h.clients.EXPECT().Update(gomock.Any(), poppyTestClientID, gomock.Any()).Return(&managementv3.UpdateClientResponseContent{}, nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "orders:read", "--require-dpop", "--out", filepath.Join(t.TempDir(), "none.json")))

		out := h.out.String()
		assert.True(t, strings.HasPrefix(out, "Agent metadata  "+poppyTestAgentURL+"\n"), out)
		assert.Contains(t, out, "    name           Meta Muse\n")
		assert.Contains(t, out, "    keys           0 ()\n")
		assert.Contains(t, out, "  ok Agent app     created  "+poppyTestClientID+"\n")
		assert.Contains(t, out, "  ok DPoP          required\n")
		assert.Contains(t, out, "  ok Agent record  created  "+poppyTestAgentID+" (in tokens as "+poppyTestAgentURL+")\n")
		assert.Contains(t, out, "  ok Grant         "+poppyTestAPI+": orders:read\n")
		assert.True(t, strings.HasSuffix(out, "\nNext\n  auth0 poppy agents show "+poppyTestAgentURL+"   to check the agent.\n"), out)
		assert.Empty(t, h.messages.String())
	})

	t.Run("a grant the app already has is said so", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)

		existing := testGrant("cgr_old", poppyTestAPI, "orders:read")
		existing.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumUser)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(existing), nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "orders:read", "--out", filepath.Join(t.TempDir(), "none.json")))

		assert.Contains(t, h.out.String(), "Grant         "+poppyTestAPI+": orders:read  (already granted)\n")
		assert.NotContains(t, h.out.String(), "FIX")
	})

	t.Run("a grant with other scopes than the ones requested says what differs and how to set them", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil)
		h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil)

		existing := testGrant("cgr_old", poppyTestAPI, "orders:read")
		existing.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumUser)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(existing), nil)

		require.NoError(t, h.root("register", poppyTestAgentURL, "--scopes", "orders:read,checkout", "--out", filepath.Join(t.TempDir(), "none.json")))

		assert.Contains(t, h.out.String(), "(already granted)\n")
		assert.Contains(t, h.out.String(), "!  the existing grant for "+poppyTestAPI+" was left as it is and lacks checkout\n")
		assert.Contains(t, h.out.String(), "FIX  auth0 api patch client-grants/cgr_old --data '{\"scope\":[\"orders:read\",\"checkout\"]}'\n")
	})
}

func TestPoppyAgentsRegisterPlanText(t *testing.T) {
	h := newPoppyAgentsHarness(t)
	h.allowRegistration()
	h.clients.EXPECT().Update(gomock.Any(), poppyTestClientID, gomock.Any()).Return(&managementv3.UpdateClientResponseContent{}, nil)

	run := h.runPromptMode(t, modeTerminal, []string{"register", poppyTestAgentURL, "--scopes", "orders:read", "--require-dpop", "--out", filepath.Join(t.TempDir(), "none.json")})

	require.NoError(t, run.err)
	assert.Equal(t, []string{"confirm"}, run.asked)
	assert.Contains(t, run.out, "\nThis will\n")
	assert.Contains(t, run.out, "  +  Agent app     create, for "+poppyTestAgentURL+"\n")
	assert.Contains(t, run.out, "  +  DPoP          required on the agent app\n")
	assert.Contains(t, run.out, `  +  Agent record  "Meta Muse"`+"\n")
	assert.Contains(t, run.out, "  +  Grant         "+poppyTestAPI+": orders:read  (skipped if the agent app already has a grant)\n")
	assert.Empty(t, run.errOut)
}

func TestPoppyAgentsList(t *testing.T) {
	setup := func(h *poppyAgentsHarness) {
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse", ExternalAgentID: testPtr(poppyTestAgentURL)}), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(testGrant("cgr_shop", poppyTestAPI, "orders:read")), nil)
	}

	t.Run("json", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		setup(h)
		h.renderer.Format = display.OutputFormatJSON

		require.NoError(t, h.root("list", "--json"))

		var got []map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		require.Len(t, got, 1)
		assert.Equal(t, poppyTestAgentURL, got[0]["url"])
		assert.Equal(t, "available", got[0]["metadata_status"])
		assert.Equal(t, false, got[0]["dpop_required"])
	})

	t.Run("table", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		setup(h)

		require.NoError(t, h.root("list"))

		assert.Contains(t, h.out.String(), "Meta Muse")
		assert.Contains(t, h.out.String(), poppyTestAgentURL)
	})
}

func TestPoppyAgentsRemove(t *testing.T) {
	t.Run("not a Poppy agent", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_other", "https://other.example/api", "refund"))

		err := h.root("remove", poppyTestAgentURL, "--force")

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "not_poppy_agent", reason)
	})

	t.Run("confirmation is required without a terminal", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))

		err := h.root("remove", poppyTestAgentURL)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "confirmation_required", reason)
		assert.Equal(t, true, details["dry_run"])
		assert.Equal(t, poppyTestAgentURL, details["url"])
		assert.Contains(t, h.out.String(), "This will remove:")
		assert.Contains(t, h.out.String(), "client grant cgr_shop ("+poppyTestAPI+")")
		assert.Empty(t, h.messages.String())
	})

	t.Run("a dry run changes nothing", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"), testGrant("cgr_other", "https://other.example/api", "refund"))

		h.json = true
		h.renderer.Format = display.OutputFormatJSON

		err := h.root("remove", poppyTestAgentURL, "--dry-run", "--remove-all-grants")

		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, true, got["dry_run"])
		assert.Len(t, got["removed"], 4)
	})

	t.Run("the output flags print the result object", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			h := newPoppyAgentsHarness(t)
			h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"), testGrant("cgr_other", "https://other.example/api", "refund"))

			err := h.root("remove", poppyTestAgentURL, "--dry-run", flag)

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
		h := newPoppyAgentsHarness(t)

		err := h.root("remove", poppyTestAgentURL, "--json", "--json-compact")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "none of the others can be")
	})

	t.Run("forced removal of the Poppy access only", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"), testGrant("cgr_other", "https://other.example/api", "refund"))
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)

		h.json = true
		h.renderer.Format = display.OutputFormatJSON

		err := h.root("remove", poppyTestAgentURL, "--force")

		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, false, got["dry_run"])
		assert.Equal(t, []any{"client grant cgr_shop (" + poppyTestAPI + ")"}, got["removed"])
		assert.Len(t, got["remaining_grants"], 1)
	})

	t.Run("forced removal of everything", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
		h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)

		err := h.root("remove", poppyTestAgentURL, "--force")

		require.NoError(t, err)
		assert.Equal(t, "Removed:\n"+
			"  ok client grant cgr_shop ("+poppyTestAPI+")\n"+
			"  ok agent record "+poppyTestAgentID+"\n"+
			"  ok agent app "+poppyTestClientID+"\n"+
			"\n"+
			"  !  This command does not revoke tokens already issued: access tokens stay valid until they expire.\n"+
			"  !  Registering the agent again creates a new app, so its users must sign in again.\n",
			h.out.String())
		assert.Empty(t, h.messages.String())
	})

	t.Run("a dry run says what would be removed and kept", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"), testGrant("cgr_other", "https://other.example/api", "refund"))

		require.NoError(t, h.root("remove", poppyTestAgentURL, "--dry-run"))

		assert.Equal(t, "This would remove:\n"+
			"  client grant cgr_shop ("+poppyTestAPI+")\n"+
			"\n"+
			"This would keep:\n"+
			"  agent record "+poppyTestAgentID+"\n"+
			"  agent app "+poppyTestClientID+"\n"+
			"\n"+
			"Other grants of the agent app stay. Remove them with:\n"+
			"  auth0 client-grants delete cgr_other   (https://other.example/api)\n",
			h.out.String())
		assert.Empty(t, h.messages.String())
	})

	t.Run("a removal that keeps the app says what was kept", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"), testGrant("cgr_other", "https://other.example/api", "refund"))
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)

		require.NoError(t, h.root("remove", poppyTestAgentURL, "--force"))

		out := h.out.String()
		assert.Contains(t, out, "Removed:\n  ok client grant cgr_shop")
		assert.Contains(t, out, "Kept:\n  -  agent record "+poppyTestAgentID+"\n  -  agent app "+poppyTestClientID+"\n")
		assert.Contains(t, out, "auth0 client-grants delete cgr_other   (https://other.example/api)")
		assert.NotContains(t, out, "Registering the agent again", "the app stays, so nothing is created again")
		assert.Empty(t, h.messages.String())
	})

	t.Run("an app that is gone is said so", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)

		require.NoError(t, h.root("remove", poppyTestAgentURL))

		assert.Contains(t, h.out.String(), "!  No agent app was found for "+poppyTestAgentURL+". It may already be removed.")
		assert.Empty(t, h.messages.String())
	})

	t.Run("the output flags leave nothing on either stream but the JSON", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			gone := newPoppyAgentsHarness(t)
			gone.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)
			require.NoError(t, gone.root("remove", poppyTestAgentURL, flag), flag)
			assert.Empty(t, gone.messages.String(), flag)
			assert.True(t, json.Valid(gone.out.Bytes()), "%s: %s", flag, gone.out.String())

			refused := newPoppyAgentsHarness(t)
			refused.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
			err := refused.root("remove", poppyTestAgentURL, flag)
			assert.Equal(t, "confirmation_required", errorReason(err), flag)
			assert.Empty(t, refused.messages.String(), flag)
			assert.Empty(t, refused.out.String(), flag)

			forced := newPoppyAgentsHarness(t)
			forced.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
			forced.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
			forced.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil)
			forced.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil)
			require.NoError(t, forced.root("remove", poppyTestAgentURL, "--force", flag), flag)
			assert.Empty(t, forced.messages.String(), flag)
			assert.True(t, json.Valid(forced.out.Bytes()), "%s: %s", flag, forced.out.String())
		}
	})

	t.Run("a partial failure keeps its details", func(t *testing.T) {
		h := newPoppyAgentsHarness(t)
		h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
		h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil)
		h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(errors.New("boom"))

		err := h.root("remove", poppyTestAgentURL, "--force")

		var partial *agents.PartialError
		require.ErrorAs(t, err, &partial)
		assert.Equal(t, []string{"delete client grant cgr_shop (" + poppyTestAPI + ")"}, partial.Applied)
		assert.Contains(t, err.Error(), "stopped partway")
	})
}

func TestPoppyError(t *testing.T) {
	forbidden := func(body string) error { return core.NewAPIError(http.StatusForbidden, nil, errors.New(body)) }

	t.Run("insufficient scope", func(t *testing.T) {
		c := &cli{}

		err := c.poppyError(forbidden(`{"errorCode":"insufficient_scope","message":"Insufficient scope, expected any of: read:agents"}`))

		assert.Equal(t, "insufficient_scope", errorReason(err))
		assert.Contains(t, err.Error(), "auth0 login --scopes read:agents")
	})

	t.Run("another 403", func(t *testing.T) {
		c := &cli{}

		err := c.poppyError(forbidden(`{"errorCode":"poppy_beta_off"}`))

		assert.EqualError(t, err, "the tenant may lack Agent as Principal or the Poppy beta; contact support")
	})

	t.Run("another 403 with debug", func(t *testing.T) {
		c := &cli{debug: true}

		err := c.poppyError(forbidden(`{"errorCode":"poppy_beta_off"}`))

		assert.EqualError(t, err, "the tenant may lack Agent as Principal or the Poppy beta; contact support (status 403)")
		assert.ErrorAs(t, err, new(*core.APIError))
	})

	t.Run("a partial failure is rewritten inside", func(t *testing.T) {
		c := &cli{}
		partial := &agents.PartialError{Applied: []string{"one"}, Remaining: []string{"two"}, Err: forbidden(`{"errorCode":"poppy_beta_off"}`)}

		err := c.poppyError(partial)

		assert.Same(t, partial, err)
		assert.ErrorContains(t, err, "the tenant may lack Agent as Principal")
	})

	t.Run("other errors pass through", func(t *testing.T) {
		c := &cli{}
		boom := errors.New("boom")

		assert.Same(t, boom, c.poppyError(boom))

		notFound := core.NewAPIError(http.StatusNotFound, nil, errors.New("missing"))
		assert.Same(t, notFound, c.poppyError(notFound))
	})
}

// runPromptMode runs an agents command of the harness in a mode and returns what it did.
func (h *poppyAgentsHarness) runPromptMode(t *testing.T, mode poppyPromptMode, args []string) poppyPromptRun {
	t.Helper()

	asked := installPoppyAsker(t, mode)
	h.agentMode = mode.agentMode

	err := h.root(append(args, mode.flags...)...)

	return poppyPromptRun{asked: *asked, out: h.out.String(), errOut: h.messages.String(), err: err}
}

// allowRegistration accepts every call that registering the test agent makes.
func (h *poppyAgentsHarness) allowRegistration() {
	h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil).AnyTimes()
	h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), nil).AnyTimes()
	h.clients.EXPECT().RegisterCimdClient(gomock.Any(), gomock.Any()).Return(&managementv3.RegisterCimdClientResponseContent{ClientID: poppyTestClientID}, nil).AnyTimes()
	h.agents.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse"}, nil).AnyTimes()
	h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil).AnyTimes()
	h.grants.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&managementv3.CreateClientGrantResponseContent{ID: testPtr("cgr_new")}, nil).AnyTimes()
}

func TestPoppyAgentsRegisterPromptRule(t *testing.T) {
	asks := []string{"confirm"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Agent app"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: "Agent metadata", json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "Agent app"},
		{name: "piped", mode: modePiped, text: "Agent app"},
		{name: "piped with --json", mode: modePipedJSON, json: true},
		{name: "agent mode", mode: modeAgent, json: true},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: "Agent metadata", json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPoppyAgentsHarness(t)
			h.allowRegistration()

			run := h.runPromptMode(t, tc.mode, []string{"register", poppyTestAgentURL, "--scopes", "orders:read", "--out", filepath.Join(t.TempDir(), "none.json")})

			assertPoppyPromptRun(t, tc, run)
		})
	}
}

func TestPoppyAgentsRegisterPromptRuleScopes(t *testing.T) {
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
			h := newPoppyAgentsHarness(t)
			h.allowRegistration()

			run := h.runPromptMode(t, tc.mode, []string{"register", poppyTestAgentURL, "--out", filepath.Join(t.TempDir(), "none.json")})

			assertPoppyPromptRun(t, tc, run)
		})
	}
}

func TestPoppyAgentsRemovePromptRule(t *testing.T) {
	const refused = "--force"

	asks := []string{"confirm"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "Removed:"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, text: "This will remove:", json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "This will remove:", reason: "confirmation_required", errText: refused},
		{name: "piped", mode: modePiped, text: "This will remove:", reason: "confirmation_required", errText: refused},
		{name: "piped with --json", mode: modePipedJSON, reason: "confirmation_required", errText: refused},
		{name: "agent mode", mode: modeAgent, reason: "confirmation_required", errText: refused},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, text: "This will remove:", json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPoppyAgentsHarness(t)
			h.expectRemovalLookup(testGrant("cgr_shop", poppyTestAPI, "orders:read"))
			h.grants.EXPECT().Delete(gomock.Any(), "cgr_shop").Return(nil).AnyTimes()
			h.agents.EXPECT().Delete(gomock.Any(), poppyTestAgentID).Return(nil).AnyTimes()
			h.clients.EXPECT().Delete(gomock.Any(), poppyTestClientID).Return(nil).AnyTimes()

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, []string{"remove", poppyTestAgentURL}))
		})
	}
}

func TestPromptPoppyGrants(t *testing.T) {
	apis := []agents.API{{
		Identifier: poppyTestAPI,
		Scopes: []agents.Scope{
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
		want         []agents.Grant
		wantReason   string
	}{
		{
			name:         "scopes that change nothing are selected to begin with",
			cfg:          cfg,
			picked:       []int{0, 2},
			wantDefaults: []int{0},
			want:         []agents.Grant{{Audience: poppyTestAPI, Scopes: []string{"orders:read", "profile"}}},
		},
		{
			name:       "no hints, nothing selected to begin with",
			picked:     []int{1},
			want:       []agents.Grant{{Audience: poppyTestAPI, Scopes: []string{"checkout"}}},
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

			screen, out := newTestPoppyScreen(t, poppyAsker{
				chooseMany: func(_ *poppyScreen, _ string, options []string, d []int) ([]int, error) {
					defaults, rows = d, options

					return test.picked, nil
				},
			}, false)

			grants, err := promptPoppyGrants(screen, apis, test.cfg)

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

func TestAskPoppyRemoval(t *testing.T) {
	tests := []struct {
		name           string
		choose         bool
		confirm        bool
		pick           int
		wantProceed    bool
		wantEverything bool
	}{
		{name: "confirmed", confirm: true, wantProceed: true, wantEverything: true},
		{name: "declined", confirm: false},
		{name: "other grants, only the Poppy access", choose: true, pick: 0, wantProceed: true},
		{name: "other grants, everything", choose: true, pick: 1, wantProceed: true, wantEverything: true},
		{name: "other grants, cancelled", choose: true, pick: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var def int

			screen, _ := newTestPoppyScreen(t, poppyAsker{
				confirm: func(*poppyScreen, string, bool) (bool, error) { return test.confirm, nil },
				choose: func(_ *poppyScreen, _ string, options []string, d int) (int, error) {
					def = d
					assert.Equal(t, []string{"Remove only the Poppy access", "Remove everything", "Cancel"}, options)

					return test.pick, nil
				},
			}, false)

			proceed, everything, err := askPoppyRemoval(screen, test.choose)

			require.NoError(t, err)
			assert.Equal(t, test.wantProceed, proceed)

			if proceed {
				assert.Equal(t, test.wantEverything, everything)
			}

			assert.Zero(t, def, "the list starts on the safest choice")
		})
	}
}

func TestPoppyRef(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("no-input", false, "")

	t.Run("an argument that is given is used", func(t *testing.T) {
		installPoppyAsker(t, modePiped)

		ref, err := poppyRef(cmd, nil, poppyAgentURL, []string{"https://agent.example/agent.json"})

		require.NoError(t, err)
		assert.Equal(t, "https://agent.example/agent.json", ref)
	})

	t.Run("without a terminal it is required", func(t *testing.T) {
		installPoppyAsker(t, modePiped)

		_, err := poppyRef(cmd, nil, poppyAgentURL, nil)

		assert.Equal(t, "missing_required_args", errorReason(err))
		assert.ErrorContains(t, err, "Agent URL")
	})

	t.Run("a terminal asks for it", func(t *testing.T) {
		installPoppyAsker(t, modeTerminal)

		screen, out := newTestPoppyScreen(t, poppyAsker{
			text: func(_ *poppyScreen, label, _ string) (string, error) {
				assert.Equal(t, "URL", label)

				return "https://agent.example/agent.json", nil
			},
		}, false)

		ref, err := poppyRef(cmd, screen, poppyAgentURL, nil)

		require.NoError(t, err)
		assert.Equal(t, "https://agent.example/agent.json", ref)
		assert.Contains(t, out.String(), "Agent URL\nURL of the agent's metadata document.\n")
	})
}

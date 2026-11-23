package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

func testAgentRecord() *managementv3.AgentResponseContent {
	return &managementv3.AgentResponseContent{AgentID: poppyTestAgentID, Name: "Meta Muse", ExternalAgentID: testPtr(poppyTestAgentURL)}
}

func forbiddenError() error {
	return core.NewAPIError(http.StatusForbidden, nil, errors.New(`{"errorCode":"insufficient_scope"}`))
}

// expectClientFilter makes the next client lookup check its filter and return the apps.
func (h *agentsHarness) expectClientFilter(t *testing.T, wantAgentID, wantURL string, apps ...*managementv3.Client) {
	t.Helper()

	h.clients.EXPECT().
		List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientListPage, error) {
			if wantAgentID != "" {
				require.NotNil(t, request.AgentID)
				assert.Equal(t, wantAgentID, *request.AgentID)
			}
			if wantURL != "" {
				require.NotNil(t, request.ExternalClientID)
				assert.Equal(t, wantURL, *request.ExternalClientID)
			}

			return testClientPage(apps...), nil
		})
}

// testM2MApp is an app linked to the test agent that is not an agent app.
func testM2MApp() *managementv3.Client {
	return &managementv3.Client{
		ClientID:                 testPtr("client_m2m"),
		Name:                     testPtr("Muse backend"),
		AgentID:                  testPtr(poppyTestAgentID),
		RequireProofOfPossession: testPtr(false),
	}
}

func (h *agentsHarness) jsonOutput() {
	h.json = true
	h.renderer.Format = display.OutputFormatJSON
}

func TestAgentsListDetails(t *testing.T) {
	atlasURL := "https://atlas.example/client.json"
	atlas := &managementv3.AgentResponseContent{AgentID: "agt_atlas", Name: "Atlas", ExternalAgentID: testPtr(atlasURL)}
	atlasApp := testAgentApp()
	atlasApp.ClientID, atlasApp.AgentID, atlasApp.ExternalClientID = testPtr("client_atlas"), testPtr("agt_atlas"), testPtr(atlasURL)
	atlasApp.RequireProofOfPossession = testPtr(true)

	t.Run("one client list and one grant list for every agent, and no resource server read", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord(), atlas), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp(), atlasApp), nil).Times(1)
		// One call for the grants of every agent, whatever their number.
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).
			Return(testGrantPage(testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read"), testUserGrant(poppyTestClientID, "cgr_other", "https://other.example/api", "refund")), nil).Times(1)

		require.NoError(t, h.root("list"))

		out := h.out.String()
		assert.Regexp(t, `ID\s+NAME\s+EXTERNAL ID\s+CREATED AT\s+CLIENT ID\s+METADATA\s+APIS`, out)
		assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+`+poppyTestClientID+`\s+available\s+`+poppyTestAPI+`, https://oth\.\.\.\s*\n`, out)
		assert.Regexp(t, `agt_atlas\s+Atlas\s+`+atlasURL+`\s+.*\s+client_atlas\s+available\s+-\s*\n`, out)
	})

	t.Run("with --poppy, one resource server read and only the grants on Poppy APIs", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord(), atlas), nil)
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer(), testOtherServer()), nil).Times(1)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp(), atlasApp), nil).Times(1)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(
			testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read"),
			testUserGrant(poppyTestClientID, "cgr_other", "https://unlisted.example/api", "refund"),
			testUserGrant("client_atlas", "cgr_atlas", "https://unlisted.example/api", "refund"),
		), nil).Times(1)

		require.NoError(t, h.root("list", "--poppy"))

		out := h.out.String()
		assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+`+poppyTestClientID+`\s+available\s+`+poppyTestAPI+`\s*\n`, out)
		assert.Regexp(t, `agt_atlas\s+Atlas\s+`+atlasURL+`\s+.*\s+client_atlas\s+available\s+-\s*\n`, out)
		assert.NotContains(t, out, "unlisted.example")
	})

	t.Run("json adds the fields after the existing ones", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.jsonOutput()
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read")), nil)

		require.NoError(t, h.root("list", "--json"))

		var got []map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		require.Len(t, got, 1)
		assert.Equal(t, poppyTestAgentID, got[0]["agent_id"])
		assert.Equal(t, poppyTestAgentURL, got[0]["external_agent_id"])
		assert.Equal(t, poppyTestClientID, got[0]["client_id"])
		assert.Equal(t, "available", got[0]["metadata_status"])
		assert.Equal(t, false, got[0]["dpop_required"])
		assert.Equal(t, []any{map[string]any{"id": "cgr_shop", "audience": poppyTestAPI, "scopes": []any{"orders:read"}}}, got[0]["apis"])
		assert.Equal(t, []any{}, got[0]["unknown_fields"])
	})

	t.Run("the APIs column counts the grants for users only", func(t *testing.T) {
		machine := testGrant("cgr_machine", "https://machine.example/api", "read")
		machine.SubjectType = testPtr(managementv3.ClientGrantSubjectTypeEnumClient)

		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(
			testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read"),
			machine,
			testGrant("cgr_untyped", "https://untyped.example/api", "read"),
		), nil).Times(1)

		require.NoError(t, h.root("list"))

		out := h.out.String()
		assert.Regexp(t, poppyTestClientID+`\s+available\s+`+poppyTestAPI+`\s*\n`, out)
		assert.NotContains(t, out, "machine.example")
		assert.NotContains(t, out, "untyped.example")
	})

	t.Run("an agent without an app has none", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(), nil)

		require.NoError(t, h.root("list"))

		assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+-\s+none\s+-\s*\n`, h.out.String())
	})

	t.Run("an agent linked to an app that is not an agent app shows that app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord(), atlas), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testM2MApp(), atlasApp), nil).Times(1)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).
			Return(testGrantPage(testUserGrant("client_m2m", "cgr_shop", poppyTestAPI, "orders:read")), nil).Times(1)

		require.NoError(t, h.root("list"))

		out := h.out.String()
		assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+client_m2m\s+-\s+`+poppyTestAPI+`\s*\n`, out)
		assert.Regexp(t, `agt_atlas\s+Atlas\s+`+atlasURL+`\s+.*\s+client_atlas\s+available\s+-\s*\n`, out)
	})

	t.Run("json for an app that is not an agent app", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.jsonOutput()
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testM2MApp()), nil)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil)

		require.NoError(t, h.root("list", "--json"))

		var got []map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, "client_m2m", got[0]["client_id"])
		assert.Equal(t, "-", got[0]["metadata_status"])
		assert.Equal(t, false, got[0]["dpop_required"])
	})

	tests := []struct {
		name       string
		args       []string
		serverErr  error
		clientsErr error
		grantErr   error
		wantRow    string
	}{
		{
			name:      "the Poppy APIs cannot be read",
			args:      []string{"--poppy"},
			serverErr: forbiddenError(),
			wantRow:   poppyTestClientID + `\s+available\s+unknown\s*\n`,
		},
		{
			name:       "the agent apps cannot be read",
			clientsErr: forbiddenError(),
			wantRow:    `unknown\s+unknown\s+unknown\s*\n`,
		},
		{
			name:     "the grants cannot be read",
			grantErr: forbiddenError(),
			wantRow:  poppyTestClientID + `\s+available\s+unknown\s*\n`,
		},
	}

	for _, test := range tests {
		t.Run(test.name+" and the list still works", func(t *testing.T) {
			h := newAgentsHarness(t)
			h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
			if len(test.args) > 0 {
				h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer()), test.serverErr)
			}
			if test.clientsErr != nil {
				h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.clientsErr)
			} else {
				h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp()), nil)
			}
			if test.serverErr == nil && test.clientsErr == nil {
				h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.grantErr)
			}

			require.NoError(t, h.root(append([]string{"list"}, test.args...)...))

			assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+`+test.wantRow, h.out.String())
		})
	}

	t.Run("a value that cannot be read is null in json, and named in unknown_fields", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.jsonOutput()
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())

		require.NoError(t, h.root("list", "--json"))

		var got []map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		for _, key := range []string{"client_id", "metadata_status", "dpop_required", "apis"} {
			value, present := got[0][key]
			assert.True(t, present, key)
			assert.Nil(t, value, key)
		}
		assert.Equal(t, []any{"client_id", "metadata_status", "dpop_required", "apis"}, got[0]["unknown_fields"])
		assert.Equal(t, poppyTestAgentID, got[0]["agent_id"])
		assert.Empty(t, h.messages.String(), "nothing but the JSON")
	})

	t.Run("a missing scope warns once, with the scope to log in with, and the list still works", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord(), atlas), nil)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(testClientPage(testAgentApp(), atlasApp), nil).Times(1)
		// One failed call for both agents, not one per agent.
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError()).Times(1)

		require.NoError(t, h.root("list"))

		assert.Regexp(t, poppyTestAgentID+`\s+Meta Muse\s+`+poppyTestAgentURL+`\s+.*\s+`+poppyTestClientID+`\s+available\s+unknown\s*\n`, h.out.String())
		assert.Equal(t, 1, strings.Count(h.messages.String(), "auth0 login --scopes"), h.messages.String())
		assert.Contains(t, h.messages.String(), "read:client_grants")
	})

	t.Run("the agents API failing still fails the command", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		err := h.root("list")

		assert.ErrorContains(t, err, "failed to list agents: boom")
	})

	t.Run("an empty list reads nothing else", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)

		require.NoError(t, h.root("list"))
	})

	t.Run("an empty list with --poppy reads nothing else either", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(), nil)

		require.NoError(t, h.root("list", "--poppy"))
	})
}

func TestAgentsShowDetails(t *testing.T) {
	expectDetails := func(h *agentsHarness) {
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read", "checkout"), testUserGrant(poppyTestClientID, "cgr_other", poppyTestOtherAPI, "refund")), nil)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return([]*managementv3.ClientCredential{{Kid: testPtr("old")}}, nil)
	}

	t.Run("by agent ID", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil).Times(1)
		h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
		expectDetails(h)

		require.NoError(t, h.root("show", poppyTestAgentID))

		out := h.out.String()
		assert.Regexp(t, `ID\s+`+poppyTestAgentID, out)
		assert.Regexp(t, `EXTERNAL ID\s+`+poppyTestAgentURL, out)
		assert.Regexp(t, `CLIENT ID\s+`+poppyTestClientID, out)
		assert.Regexp(t, `METADATA STATUS\s+available`, out)
		assert.Regexp(t, `DPOP REQUIRED\s+not required`, out)
		assert.Regexp(t, `APIS\s+`+poppyTestAPI+`: orders:read checkout\s+`+poppyTestOtherAPI+`: refund`, out)
		assert.Regexp(t, `METADATA CHECK\s+valid`, out)
		assert.Regexp(t, `KEYS\s+changed since registration`, out)
		assert.Contains(t, h.messages.String(), "Run `auth0 agents register "+poppyTestAgentURL+"` again.")
	})

	t.Run("with --poppy, one resource server read and only the grants on Poppy APIs", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil).Times(1)
		h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
		h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(testShopServer(), testOtherServer()), nil).Times(1)
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(testUserGrant(poppyTestClientID, "cgr_shop", poppyTestAPI, "orders:read", "checkout"), testUserGrant(poppyTestClientID, "cgr_x", "https://unlisted.example/api", "refund")), nil)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return([]*managementv3.ClientCredential{{Kid: testPtr("old")}}, nil)

		require.NoError(t, h.root("show", poppyTestAgentID, "--poppy"))

		out := h.out.String()
		assert.Regexp(t, `APIS\s+`+poppyTestAPI+`: orders:read checkout\s*\n`, out)
		assert.NotContains(t, out, "unlisted.example")
	})

	t.Run("by agent URL, without reading the record twice", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil).Times(1)
		h.expectClientFilter(t, "", poppyTestAgentURL, testAgentApp())
		expectDetails(h)

		require.NoError(t, h.root("show", poppyTestAgentURL))

		assert.Regexp(t, `ID\s+`+poppyTestAgentID, h.out.String())
		assert.Regexp(t, `CLIENT ID\s+`+poppyTestClientID, h.out.String())
	})

	t.Run("json adds the fields only", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.jsonOutput()
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
		expectDetails(h)

		require.NoError(t, h.root("show", poppyTestAgentID, "--json"))

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Equal(t, poppyTestAgentID, got["agent_id"])
		assert.Equal(t, "Meta Muse", got["name"])
		assert.Equal(t, poppyTestClientID, got["client_id"])
		assert.Equal(t, "available", got["metadata_status"])
		assert.Equal(t, false, got["dpop_required"])
		assert.Equal(t, false, got["keys_current"])
		assert.Equal(t, map[string]any{"valid": true, "violations": []any{}, "warnings": []any{}}, got["metadata_check"])
		assert.Equal(t, []any{}, got["notes"])
		assert.Empty(t, h.messages.String(), "nothing but the JSON")
	})

	t.Run("an agent that is not found", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(nil, errors.New("404 Not Found: inexistent_agent"))

		err := h.root("show", poppyTestAgentID)

		assert.ErrorContains(t, err, `failed to get agent "`+poppyTestAgentID+`": 404 Not Found: inexistent_agent`)
	})

	t.Run("an URL no agent app and no agent record has", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectClientFilter(t, "", poppyTestAgentURL)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(&managementv3.AgentResponseContent{AgentID: "agt_other", ExternalAgentID: testPtr("https://other.example/agent.json")}), nil)

		err := h.root("show", poppyTestAgentURL)

		assert.ErrorContains(t, err, `failed to get agent "`+poppyTestAgentURL+`": no agent found for this URL`)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "agent_not_found", reason)
		assert.Equal(t, "auth0 agents register "+poppyTestAgentURL, details["next"])
	})

	t.Run("an URL only an agent record has", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectClientFilter(t, "", poppyTestAgentURL)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(testAgentPage(testAgentRecord()), nil)
		h.expectClientFilter(t, poppyTestAgentID, "")

		require.NoError(t, h.root("show", poppyTestAgentURL))

		out := h.out.String()
		assert.Regexp(t, `ID\s+`+poppyTestAgentID, out)
		assert.Regexp(t, `EXTERNAL ID\s+`+poppyTestAgentURL, out)
		assert.Regexp(t, `CLIENT ID\s+-`, out)
		assert.Regexp(t, `METADATA STATUS\s+none`, out)
		assert.Regexp(t, `APIS\s+-`, out)
		assert.Regexp(t, `METADATA CHECK\s+-`, out)
	})

	t.Run("an URL the agent records cannot be listed for", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectClientFilter(t, "", poppyTestAgentURL)
		h.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())

		err := h.root("show", poppyTestAgentURL)

		assert.ErrorContains(t, err, `failed to get agent "`+poppyTestAgentURL+`": failed to list agents`)
	})

	t.Run("an app that is not an agent app is shown without a metadata check", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.expectClientFilter(t, poppyTestAgentID, "", testM2MApp())
		h.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: testPtr("client_m2m")}).
			Return(testGrantPage(testUserGrant("client_m2m", "cgr_shop", poppyTestAPI, "orders:read")), nil)

		require.NoError(t, h.root("show", poppyTestAgentID))

		out := h.out.String()
		assert.Regexp(t, `CLIENT ID\s+client_m2m`, out)
		assert.Regexp(t, `METADATA STATUS\s+-\s*\n`, out)
		assert.Regexp(t, `DPOP REQUIRED\s+not required`, out)
		assert.Regexp(t, `APIS\s+`+poppyTestAPI+`: orders:read`, out)
		assert.Regexp(t, `METADATA CHECK\s+-\s*\n`, out)
		assert.Regexp(t, `KEYS\s+-\s*\n`, out)
		assert.NotContains(t, h.messages.String(), "metadata document")
	})

	t.Run("an agent app that has no agent record", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.expectClientFilter(t, "", poppyTestAgentURL, testAgentApp())
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(nil, core.NewAPIError(http.StatusNotFound, nil, errors.New("gone")))

		err := h.root("show", poppyTestAgentURL)

		assert.ErrorContains(t, err, "no agent record is linked to the agent app "+poppyTestClientID)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "agent_not_linked", reason)
		assert.Equal(t, "auth0 agents register "+poppyTestAgentURL, details["next"])
	})

	t.Run("the lookup of an URL failing fails the command", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())

		err := h.root("show", poppyTestAgentURL)

		assert.ErrorContains(t, err, "failed to look up "+poppyTestAgentURL)
	})

	t.Run("no agent app for an ID", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.expectClientFilter(t, poppyTestAgentID, "")

		require.NoError(t, h.root("show", poppyTestAgentID))

		out := h.out.String()
		assert.Regexp(t, `CLIENT ID\s+-`, out)
		assert.Regexp(t, `METADATA STATUS\s+none`, out)
		assert.Regexp(t, `APIS\s+-`, out)
		assert.Regexp(t, `METADATA CHECK\s+-`, out)
	})

	t.Run("json for a value that cannot be read is null, named in unknown_fields", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.jsonOutput()
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError()).Times(1)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return(nil, nil)

		require.NoError(t, h.root("show", poppyTestAgentID, "--json"))

		var got map[string]any
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &got))
		assert.Contains(t, got, "apis")
		assert.Nil(t, got["apis"])
		assert.Equal(t, []any{"apis"}, got["unknown_fields"])
		assert.Equal(t, poppyTestClientID, got["client_id"])
		assert.Empty(t, h.messages.String(), "nothing but the JSON")
	})

	t.Run("a missing scope warns once, with the scope to log in with", func(t *testing.T) {
		h := newAgentsHarness(t)
		h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
		h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
		h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError()).Times(1)
		h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
		h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return(nil, forbiddenError())

		require.NoError(t, h.root("show", poppyTestAgentID))

		assert.Equal(t, 1, strings.Count(h.messages.String(), "auth0 login --scopes"), h.messages.String())
		assert.Contains(t, h.messages.String(), "read:client_grants")
		assert.Contains(t, h.messages.String(), "read:client_credentials")
	})

	degraded := []struct {
		name  string
		args  []string
		setup func(h *agentsHarness)
		want  map[string]string
		note  string
	}{
		{
			name: "the agent app cannot be looked up",
			setup: func(h *agentsHarness) {
				h.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())
			},
			want: map[string]string{"CLIENT ID": "unknown", "METADATA STATUS": "unknown", "DPOP REQUIRED": "unknown", "APIS": "unknown", "METADATA CHECK": "unknown", "KEYS": "unknown"},
			note: "The agent app could not be read, so the metadata document was not checked",
		},
		{
			name: "the Poppy APIs cannot be read",
			args: []string{"--poppy"},
			setup: func(h *agentsHarness) {
				h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
				h.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())
				h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
				h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return(nil, nil)
			},
			want: map[string]string{"CLIENT ID": poppyTestClientID, "APIS": "unknown", "METADATA CHECK": "valid"},
		},
		{
			name: "the document cannot be checked",
			setup: func(h *agentsHarness) {
				h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
				h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(testGrantPage(), nil)
				h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())
			},
			want: map[string]string{"APIS": "-", "METADATA CHECK": "unknown", "KEYS": "unknown"},
			note: "The metadata document could not be checked",
		},
		{
			name: "the stored keys cannot be read",
			setup: func(h *agentsHarness) {
				h.expectClientFilter(t, poppyTestAgentID, "", testAgentApp())
				h.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, forbiddenError())
				h.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(validPreview(), nil)
				h.credentials.EXPECT().List(gomock.Any(), poppyTestClientID).Return(nil, forbiddenError())
			},
			want: map[string]string{"APIS": "unknown", "METADATA CHECK": "valid", "KEYS": "unknown"},
			note: "The keys Auth0 stored could not be read",
		},
	}

	for _, test := range degraded {
		t.Run(test.name+" and the agent is still shown", func(t *testing.T) {
			h := newAgentsHarness(t)
			h.agents.EXPECT().Read(gomock.Any(), poppyTestAgentID).Return(testAgentRecord(), nil)
			test.setup(h)

			require.NoError(t, h.root(append([]string{"show", poppyTestAgentID}, test.args...)...))

			assert.Regexp(t, `NAME\s+Meta Muse`, h.out.String())
			for key, want := range test.want {
				assert.Regexp(t, key+`\s+`+want+`\s*\n`, h.out.String(), key)
			}
			if test.note != "" {
				assert.Contains(t, h.messages.String(), test.note)
			}
		})
	}
}

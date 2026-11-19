package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

const (
	museURL   = "https://muse.example/agent.json"
	atlasURL  = "https://atlas.example/client.json"
	shopAPI   = "https://api.shop.example"
	otherAPI  = "https://other.example/api"
	museApp   = "client_muse"
	museAgent = "agt_muse"
)

type fixture struct {
	clients     *mock.MockClientAPIV3
	agents      *mock.MockAgentsAPI
	grants      *mock.MockClientGrantAPIV3
	servers     *mock.MockResourceServerAPIV3
	credentials *mock.MockClientCredentialAPIV3
	svc         *Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	f := &fixture{
		clients:     mock.NewMockClientAPIV3(ctrl),
		agents:      mock.NewMockAgentsAPI(ctrl),
		grants:      mock.NewMockClientGrantAPIV3(ctrl),
		servers:     mock.NewMockResourceServerAPIV3(ctrl),
		credentials: mock.NewMockClientCredentialAPIV3(ctrl),
	}
	f.svc = &Service{
		Clients:         f.clients,
		Agents:          f.agents,
		Credentials:     f.credentials,
		Grants:          f.grants,
		ResourceServers: f.servers,
	}

	return f
}

func known[T any](value T) Maybe[T] { return Known(value) }

func ptr[T any](value T) *T { return &value }

func newPage[C comparable, T, R any](items ...T) *core.Page[C, T, R] {
	return &core.Page[C, T, R]{
		Results:      items,
		NextPageFunc: func(context.Context) (*core.Page[C, T, R], error) { return nil, core.ErrNoPages },
	}
}

func clientPage(items ...*managementv3.Client) *auth0.ClientListPage {
	return newPage[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent](items...)
}

func agentPage(items ...*managementv3.AgentResponseContent) *auth0.AgentListPage {
	return newPage[*string, *managementv3.AgentResponseContent, *managementv3.ListAgentsResponseContent](items...)
}

func grantPage(items ...*managementv3.ClientGrantResponseContent) *auth0.ClientGrantPage {
	return newPage[*string, *managementv3.ClientGrantResponseContent, *managementv3.ListClientGrantPaginatedResponseContent](items...)
}

func serverPage(items ...*managementv3.ResourceServer) *auth0.ResourceServerPage {
	return newPage[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent](items...)
}

// apiError builds the error the SDK returns for a failed Management API call.
func apiError(status int, body string) error {
	return core.NewAPIError(status, nil, errors.New(body))
}

func shopServer() *managementv3.ResourceServer {
	return &managementv3.ResourceServer{
		ID:                          ptr("rs_shop"),
		Identifier:                  ptr(shopAPI),
		UseForPersonalAgentProtocol: ptr(true),
		Scopes: []*managementv3.ResourceServerScope{
			{Value: "orders:read", Description: ptr("See your orders")},
			{Value: "checkout", Description: ptr("Place orders")},
		},
	}
}

func otherServer() *managementv3.ResourceServer {
	return &managementv3.ResourceServer{ID: ptr("rs_other"), Identifier: ptr(otherAPI)}
}

func shopAPIValue() API {
	return API{
		ID:         "rs_shop",
		Identifier: shopAPI,
		Scopes:     []Scope{{Name: "orders:read", Description: "See your orders"}, {Name: "checkout", Description: "Place orders"}},
	}
}

func agentApp(clientID, url, agentID string) *managementv3.Client {
	return &managementv3.Client{
		ClientID:               ptr(clientID),
		Name:                   ptr("App " + clientID),
		ExternalClientID:       ptr(url),
		ExternalMetadataType:   ptr(managementv3.ClientExternalMetadataTypeEnumCimd),
		ExternalMetadataStatus: ptr(managementv3.ClientExternalMetadataStatusEnumAvailable),
		AgentID:                ptr(agentID),
	}
}

func agentRecord(agentID, name, url string) *managementv3.AgentResponseContent {
	return &managementv3.AgentResponseContent{AgentID: agentID, Name: name, ExternalAgentID: ptr(url)}
}

func TestMaybe(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "known bool", value: Known(true), want: `true`},
		{name: "known list", value: Known([]string{"checkout"}), want: `["checkout"]`},
		{name: "unknown bool", value: Maybe[bool]{}, want: `null`},
		{name: "unknown list", value: Maybe[[]string]{}, want: `null`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.value)

			require.NoError(t, err)
			assert.JSONEq(t, test.want, string(got))
		})
	}
}

func TestRunSteps(t *testing.T) {
	ok := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errors.New("boom") }

	steps := func(runs ...func(context.Context) error) []Step {
		names := []string{"one", "two", "three"}
		out := make([]Step, len(runs))
		for i, run := range runs {
			out[i] = Step{Verb: "do", Noun: names[i], Run: run}
		}
		return out
	}

	t.Run("all steps pass", func(t *testing.T) {
		assert.NoError(t, RunSteps(t.Context(), steps(ok, ok, ok)))
	})

	t.Run("the first step failing is a plain error", func(t *testing.T) {
		err := RunSteps(t.Context(), steps(fail, ok))

		assert.EqualError(t, err, "failed to do one: boom")
		var partial *PartialError
		assert.False(t, errors.As(err, &partial))
	})

	t.Run("a later step failing is partial", func(t *testing.T) {
		err := RunSteps(t.Context(), steps(ok, fail, ok))

		var partial *PartialError
		require.ErrorAs(t, err, &partial)
		assert.Equal(t, []string{"do one"}, partial.Applied)
		assert.Equal(t, "do two", partial.Failed)
		assert.Equal(t, []string{"do three"}, partial.Remaining)
		assert.EqualError(t, err, "stopped partway: failed to do two: boom\n  applied: do one\n  failed: do two\n  remaining: do three")
	})

	t.Run("the last step failing leaves nothing remaining", func(t *testing.T) {
		err := RunSteps(t.Context(), steps(ok, ok, fail))

		var partial *PartialError
		require.ErrorAs(t, err, &partial)
		assert.Equal(t, []string{"do one", "do two"}, partial.Applied)
		assert.Equal(t, "do three", partial.Failed)
		assert.Empty(t, partial.Remaining)
		assert.EqualError(t, err, "stopped partway: failed to do three: boom\n  applied: do one, do two\n  failed: do three")
	})
}

// core409 is the error the SDK returns for an agent that is already registered.
func core409() error { return core.NewAPIError(409, nil, errors.New("conflict")) }

// grantOf is a grant of the agent app of the tests.
func grantOf(id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	return &managementv3.ClientGrantResponseContent{ID: ptr(id), ClientID: ptr(museApp), Audience: ptr(audience), Scope: scopes}
}

// grantFor moves a grant to another app.
func grantFor(clientID string, grant *managementv3.ClientGrantResponseContent) *managementv3.ClientGrantResponseContent {
	moved := *grant
	moved.ClientID = ptr(clientID)

	return &moved
}

// userGrantOf is a grant for users, the kind that lets an agent act for a person.
func userGrantOf(id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	grant := grantOf(id, audience, scopes...)
	grant.SubjectType = ptr(managementv3.ClientGrantSubjectTypeEnumUser)

	return grant
}

// clientGrantOf is a grant of the app itself, as a machine-to-machine app has.
func clientGrantOf(id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	grant := grantOf(id, audience, scopes...)
	grant.SubjectType = ptr(managementv3.ClientGrantSubjectTypeEnumClient)

	return grant
}

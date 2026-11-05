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
	agents      *mock.MockAgentAPIV3
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
		agents:      mock.NewMockAgentAPIV3(ctrl),
		grants:      mock.NewMockClientGrantAPIV3(ctrl),
		servers:     mock.NewMockResourceServerAPIV3(ctrl),
		credentials: mock.NewMockClientCredentialAPIV3(ctrl),
	}
	f.svc = &Service{
		Clients:         f.clients,
		Agents:          f.agents,
		Grants:          f.grants,
		ResourceServers: f.servers,
		Credentials:     f.credentials,
	}

	return f
}

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

func agentPage(items ...*managementv3.AgentResponseContent) *auth0.AgentPage {
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

func grantOf(id, audience string, scopes ...string) *managementv3.ClientGrantResponseContent {
	return &managementv3.ClientGrantResponseContent{ID: ptr(id), Audience: ptr(audience), Scope: scopes}
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

func TestMaybe(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "known bool", value: known(true), want: `true`},
		{name: "known list", value: known([]Grant{{Audience: shopAPI, Scopes: []string{"checkout"}}}), want: `[{"audience":"https://api.shop.example","scopes":["checkout"]}]`},
		{name: "unknown bool", value: Maybe[bool]{}, want: `"unknown"`},
		{name: "unknown list", value: Maybe[[]Grant]{}, want: `"unknown"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.value)

			require.NoError(t, err)
			assert.JSONEq(t, test.want, string(got))
		})
	}
}

func TestPoppyAPIs(t *testing.T) {
	tests := []struct {
		name      string
		servers   []*managementv3.ResourceServer
		listErr   error
		picked    []string
		want      []string
		wantError string
	}{
		{name: "every Poppy API by default", servers: []*managementv3.ResourceServer{otherServer(), shopServer()}, want: []string{shopAPI}},
		{name: "picked by identifier", servers: []*managementv3.ResourceServer{shopServer()}, picked: []string{shopAPI}, want: []string{shopAPI}},
		{name: "picked by id", servers: []*managementv3.ResourceServer{shopServer()}, picked: []string{"rs_shop"}, want: []string{shopAPI}},
		{name: "picked twice", servers: []*managementv3.ResourceServer{shopServer()}, picked: []string{"rs_shop", shopAPI}, want: []string{shopAPI}},
		{name: "not a Poppy API", servers: []*managementv3.ResourceServer{otherServer(), shopServer()}, picked: []string{otherAPI}, wantError: `"https://other.example/api" is not a Poppy API`},
		{name: "no Poppy API", servers: []*managementv3.ResourceServer{otherServer()}, wantError: "no Poppy API found. Run: auth0 poppy setup"},
		{name: "list fails", listErr: errors.New("boom"), wantError: "failed to list APIs: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(test.servers...), test.listErr)

			apis, err := f.svc.PoppyAPIs(t.Context(), test.picked)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				assert.Equal(t, test.name == "no Poppy API", errors.Is(err, ErrNoPoppyAPI))
				return
			}

			require.NoError(t, err)
			var got []string
			for _, api := range apis {
				got = append(got, api.Identifier)
			}
			assert.Equal(t, test.want, got)
		})
	}

	t.Run("reads the scopes", func(t *testing.T) {
		f := newFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(shopServer()), nil)

		apis, err := f.svc.PoppyAPIs(t.Context(), nil)

		require.NoError(t, err)
		assert.Equal(t, []API{shopAPIValue()}, apis)
	})
}

func TestRunSteps(t *testing.T) {
	ok := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errors.New("boom") }

	steps := func(runs ...func(context.Context) error) []step {
		names := []string{"one", "two", "three"}
		out := make([]step, len(runs))
		for i, run := range runs {
			out[i] = step{verb: "do", noun: names[i], run: run}
		}
		return out
	}

	t.Run("all steps pass", func(t *testing.T) {
		assert.NoError(t, runSteps(t.Context(), steps(ok, ok, ok)))
	})

	t.Run("the first step failing is a plain error", func(t *testing.T) {
		err := runSteps(t.Context(), steps(fail, ok))

		assert.EqualError(t, err, "failed to do one: boom")
		var partial *PartialError
		assert.False(t, errors.As(err, &partial))
	})

	t.Run("a later step failing is partial", func(t *testing.T) {
		err := runSteps(t.Context(), steps(ok, fail, ok))

		var partial *PartialError
		require.ErrorAs(t, err, &partial)
		assert.Equal(t, []string{"do one"}, partial.Applied)
		assert.Equal(t, []string{"do two", "do three"}, partial.Remaining)
		assert.EqualError(t, err, "stopped partway: failed to do two: boom\n  applied: do one\n  remaining: do two, do three")
		assert.JSONEq(t, `{"code":"partial","applied":["do one"],"remaining":["do two","do three"]}`, string(partial.ErrorDetails()))
	})
}

func TestGrantsOfReadsTheSubjectType(t *testing.T) {
	f := newFixture(t)
	f.grants.EXPECT().
		List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp)}).
		Return(grantPage(userGrantOf("cgr_user", shopAPI, "orders:read"), clientGrantOf("cgr_m2m", shopAPI, "orders:read"), grantOf("cgr_none", shopAPI)), nil)

	got, err := f.svc.grantsOf(t.Context(), museApp)

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.True(t, got[0].IsUserGrant())
	assert.False(t, got[1].IsUserGrant())
	assert.False(t, got[2].IsUserGrant())
}

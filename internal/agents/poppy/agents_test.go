package poppy

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
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
		Service: &coreagents.Service{
			Clients:         f.clients,
			Agents:          f.agents,
			Credentials:     f.credentials,
			Grants:          f.grants,
			ResourceServers: f.servers,
		},
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

func serverPage(items ...*managementv3.ResourceServer) *auth0.ResourceServerPage {
	return newPage[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent](items...)
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

func shopAPIValue() coreagents.API {
	return coreagents.API{
		ID:         "rs_shop",
		Identifier: shopAPI,
		Scopes:     []coreagents.Scope{{Name: "orders:read", Description: "See your orders"}, {Name: "checkout", Description: "Place orders"}},
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
		assert.Equal(t, []coreagents.API{shopAPIValue()}, apis)
	})
}

func TestAudiences(t *testing.T) {
	t.Run("the identifiers of the Poppy APIs only", func(t *testing.T) {
		f := newFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(otherServer(), shopServer()), nil)

		got, err := f.svc.Audiences(t.Context())

		require.NoError(t, err)
		assert.Equal(t, map[string]bool{shopAPI: true}, got)
	})

	t.Run("the list fails", func(t *testing.T) {
		f := newFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		_, err := f.svc.Audiences(t.Context())

		assert.EqualError(t, err, "failed to list APIs: boom")
	})

	t.Run("without the resource servers API", func(t *testing.T) {
		_, err := (&Service{Service: &coreagents.Service{}}).Audiences(t.Context())

		assert.Error(t, err)
	})
}

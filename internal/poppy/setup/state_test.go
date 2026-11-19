package setup

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

const (
	shopAPI   = "https://api.shop.example"
	shopApp   = "W1x9aBcD2eFgH3iJ"
	tenantURL = "tenant.us.auth0.com"
)

func ptr[T any](value T) *T { return &value }

// chain builds a list response made of the given pages.
func chain[C comparable, T, R any](pages ...[]T) *core.Page[C, T, R] {
	p := &core.Page[C, T, R]{
		Results:      pages[0],
		NextPageFunc: func(context.Context) (*core.Page[C, T, R], error) { return nil, core.ErrNoPages },
	}

	if len(pages) > 1 {
		next := chain[C, T, R](pages[1:]...)
		p.NextPageFunc = func(context.Context) (*core.Page[C, T, R], error) { return next, nil }
	}

	return p
}

func serverPages(pages ...[]*managementv3.ResourceServer) *auth0.ResourceServerPage {
	return chain[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent](pages...)
}

func clientPages(pages ...[]*managementv3.Client) *auth0.ClientListPage {
	return chain[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent](pages...)
}

func notFound() error {
	return core.NewAPIError(http.StatusNotFound, nil, errors.New("not found"))
}

func shopServer() *managementv3.ResourceServer {
	return &managementv3.ResourceServer{
		ID:                          ptr("rs_shop"),
		Identifier:                  ptr(shopAPI),
		AllowOfflineAccess:          ptr(true),
		UseForPersonalAgentProtocol: ptr(true),
		AgentSubjectClaims:          ptr(managementv3.ResourceServerAgentSubjectClaimsResponseEnumAuth0V1),
		Scopes: []*managementv3.ResourceServerScope{
			{Value: "orders:read", Description: ptr("See your orders and their status. Read only.")},
			{Value: "checkout"},
		},
		SubjectTypeAuthorization: &managementv3.ResourceServerSubjectTypeAuthorization{
			AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{
				Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant),
			},
		},
	}
}

func shopClient() *managementv3.GetClientResponseContent {
	return &managementv3.GetClientResponseContent{
		ClientID:                      ptr(shopApp),
		Name:                          ptr("Shop Web"),
		IsFirstParty:                  ptr(true),
		InitiateLoginURI:              ptr("https://shop.example/login"),
		AllowPersonalAgentBrowserJoin: ptr(true),
	}
}

type readerFixture struct {
	tenant  *mock.MockTenantSettingsAPIV3
	domains *mock.MockCustomDomainAPI
	servers *mock.MockResourceServerAPIV3
	clients *mock.MockClientAPIV3
	reader  *Reader
}

func newReaderFixture(t *testing.T) *readerFixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	f := &readerFixture{
		tenant:  mock.NewMockTenantSettingsAPIV3(ctrl),
		domains: mock.NewMockCustomDomainAPI(ctrl),
		servers: mock.NewMockResourceServerAPIV3(ctrl),
		clients: mock.NewMockClientAPIV3(ctrl),
	}
	f.reader = &Reader{
		TenantDomain: tenantURL,
		Tenant:       f.tenant,
		Domains:      f.domains,
		Servers:      f.servers,
		Clients:      f.clients,
	}

	return f
}

func (f *readerFixture) expectTenant() {
	f.tenant.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&managementv3.GetTenantSettingsResponseContent{
		ClientIDMetadataDocumentSupported:          ptr(true),
		AuthorizationResponseIssParameterSupported: ptr(false),
		ResourceParameterProfile:                   ptr(managementv3.TenantSettingsResourceParameterProfileCompatibility),
		DefaultAudience:                            ptr(shopAPI),
		FriendlyName:                               ptr("Shop"),
		PersonalAgentProtocolCompanyDomains:        []string{"https://shop.example"},
	}, nil)
}

func readyDomain(domain string, isDefault bool) *management.CustomDomain {
	return &management.CustomDomain{Domain: ptr(domain), Status: ptr("ready"), IsDefault: ptr(isDefault)}
}

func TestReadByIdentifierAndID(t *testing.T) {
	f := newReaderFixture(t)
	f.expectTenant()
	f.domains.EXPECT().List(gomock.Any()).Return([]*management.CustomDomain{readyDomain("auth.shop.example", true)}, nil)
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Times(2).Return(serverPages([]*managementv3.ResourceServer{shopServer()}), nil)
	f.clients.EXPECT().Get(gomock.Any(), shopApp, gomock.Any()).Return(shopClient(), nil)

	state, err := f.reader.Read(t.Context(), []string{shopAPI, shopAPI}, shopApp)

	require.NoError(t, err)
	assert.Equal(t, Tenant{
		CIMD:                     true,
		ResourceParameterProfile: "compatibility",
		DefaultAudience:          shopAPI,
		FriendlyName:             "Shop",
		CompanyDomains:           []string{"https://shop.example"},
	}, state.Tenant)
	assert.Equal(t, tenantURL, state.Domain)
	assert.Equal(t, "auth.shop.example", state.CustomDomain)
	assert.Equal(t, "https://auth.shop.example/", state.Issuer())
	assert.False(t, state.usesCanonicalDomain())
	assert.Equal(t, []API{{
		ID:                 "rs_shop",
		Identifier:         shopAPI,
		Scopes:             []Scope{{Name: "orders:read", Description: "See your orders and their status. Read only."}, {Name: "checkout"}},
		AllowOfflineAccess: true,
		AgentSubjectClaims: "auth0-v1",
		Poppy:              true,
		GuestPolicy:        "require_client_grant",
	}}, state.APIs, "the same API asked for twice is read once")
	assert.Equal(t, App{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true}, state.App)
}

func TestReadKeepsTheOtherSubjectTypePolicies(t *testing.T) {
	server := shopServer()
	server.SubjectTypeAuthorization.User = &managementv3.ResourceServerSubjectTypeAuthorizationUser{
		Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationUserPolicyEnumDenyAll),
	}
	server.SubjectTypeAuthorization.Client = &managementv3.ResourceServerSubjectTypeAuthorizationClient{
		Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationClientPolicyEnumRequireClientGrant),
	}

	f := newReaderFixture(t)
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{server}), nil)

	api, err := f.reader.findAPI(t.Context(), shopAPI)

	require.NoError(t, err)
	assert.Equal(t, "deny_all", api.UserPolicy)
	assert.Equal(t, "require_client_grant", api.ClientPolicy)
	assert.Equal(t, "require_client_grant", api.GuestPolicy)
}

func TestFindAPIUsesOnlyTheAPIAskedFor(t *testing.T) {
	other := &managementv3.ResourceServer{ID: ptr("rs_other"), Identifier: ptr("https://other.example")}

	t.Run("a filter that is ignored", func(t *testing.T) {
		f := newReaderFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{other, shopServer()}), nil)

		api, err := f.reader.findAPI(t.Context(), shopAPI)

		require.NoError(t, err)
		assert.Equal(t, "rs_shop", api.ID)
	})

	t.Run("nothing matches", func(t *testing.T) {
		f := newReaderFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Times(2).Return(serverPages([]*managementv3.ResourceServer{other}), nil)

		_, err := f.reader.findAPI(t.Context(), shopAPI)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, "api_not_found", input.Reason)
	})

	t.Run("an ID when the filter is ignored", func(t *testing.T) {
		f := newReaderFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Times(2).Return(serverPages([]*managementv3.ResourceServer{other, shopServer()}), nil)

		api, err := f.reader.findAPI(t.Context(), "rs_shop")

		require.NoError(t, err)
		assert.Equal(t, shopAPI, api.Identifier)
	})
}

func TestFindAppByNameWhenTheReferenceIsNotAnID(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		getErr  error
		wantErr string
		byName  bool
	}{
		{name: "404", getErr: notFound(), byName: true},
		{name: "400", getErr: core.NewAPIError(http.StatusBadRequest, nil, errors.New("invalid id")), byName: true},
		{name: "403", getErr: core.NewAPIError(http.StatusForbidden, nil, errors.New("forbidden")), wantErr: "failed to read the app Shop Web"},
		{name: "500", getErr: core.NewAPIError(http.StatusInternalServerError, nil, errors.New("down")), wantErr: "failed to read the app Shop Web"},
		{name: "not an API error", getErr: boom, wantErr: "boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			f.clients.EXPECT().Get(gomock.Any(), "Shop Web", gomock.Any()).Return(nil, test.getErr)

			if test.byName {
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPages([]*managementv3.Client{{ClientID: ptr(shopApp), Name: ptr("Shop Web")}}), nil)
				f.clients.EXPECT().Get(gomock.Any(), shopApp, gomock.Any()).Return(shopClient(), nil)
			}

			app, err := f.reader.FindApp(t.Context(), "Shop Web")

			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, shopApp, app.ClientID)
		})
	}
}

func TestTenantHosts(t *testing.T) {
	f := newReaderFixture(t)
	f.domains.EXPECT().List(gomock.Any()).Return([]*management.CustomDomain{readyDomain("auth.shop.example", true), readyDomain("login.shop.example", false)}, nil)

	hosts, err := f.reader.TenantHosts(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{tenantURL, "auth.shop.example", "login.shop.example"}, hosts)

	f = newReaderFixture(t)
	f.domains.EXPECT().List(gomock.Any()).Return(nil, errors.New("boom"))

	_, err = f.reader.TenantHosts(t.Context())

	require.ErrorContains(t, err, "failed to list the custom domains")
}

func TestCustomDomainsAreListedOncePerRun(t *testing.T) {
	f := newReaderFixture(t)
	f.expectTenant()
	f.domains.EXPECT().List(gomock.Any()).Return([]*management.CustomDomain{readyDomain("auth.shop.example", true)}, nil).Times(1)

	hosts, err := f.reader.TenantHosts(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{tenantURL, "auth.shop.example"}, hosts)

	state, err := f.reader.Read(t.Context(), nil, "")
	require.NoError(t, err)
	assert.Equal(t, "auth.shop.example", state.CustomDomain)
}

func TestFailedCustomDomainsReadIsTriedAgain(t *testing.T) {
	f := newReaderFixture(t)
	gomock.InOrder(
		f.domains.EXPECT().List(gomock.Any()).Return(nil, errors.New("boom")),
		f.domains.EXPECT().List(gomock.Any()).Return([]*management.CustomDomain{readyDomain("auth.shop.example", true)}, nil),
	)

	_, err := f.reader.TenantHosts(t.Context())
	require.ErrorContains(t, err, "failed to list the custom domains")

	hosts, err := f.reader.TenantHosts(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{tenantURL, "auth.shop.example"}, hosts)
}

func TestReadFindsAPIByIDAndAppByName(t *testing.T) {
	f := newReaderFixture(t)
	f.expectTenant()
	f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)

	other := &managementv3.ResourceServer{ID: ptr("rs_other"), Identifier: ptr("https://other.example")}
	gomock.InOrder(
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *managementv3.ListResourceServerRequestParameters, _ ...any) (*auth0.ResourceServerPage, error) {
				require.Len(t, request.Identifiers, 1)
				assert.Equal(t, "rs_shop", *request.Identifiers[0])

				return serverPages([]*managementv3.ResourceServer{}), nil
			}),
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(
			serverPages([]*managementv3.ResourceServer{other}, []*managementv3.ResourceServer{shopServer()}), nil),
	)

	gomock.InOrder(
		f.clients.EXPECT().Get(gomock.Any(), "Shop Web", gomock.Any()).Return(nil, notFound()),
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...any) (*auth0.ClientListPage, error) {
				assert.Nil(t, request.Q, "the API has no name filter: q accepts only client_grant queries")

				return clientPages(
					[]*managementv3.Client{{ClientID: ptr("c_partial"), Name: ptr("Shop Web Staging")}},
					[]*managementv3.Client{{ClientID: ptr(shopApp), Name: ptr("Shop Web")}},
				), nil
			}),
		f.clients.EXPECT().Get(gomock.Any(), shopApp, gomock.Any()).Return(shopClient(), nil),
	)
	state, err := f.reader.Read(t.Context(), []string{"rs_shop"}, "Shop Web")

	require.NoError(t, err)
	assert.Equal(t, "rs_shop", state.APIs[0].ID)
	assert.Equal(t, shopApp, state.App.ClientID)
	assert.True(t, state.usesCanonicalDomain())
	assert.Equal(t, "https://"+tenantURL+"/", state.Issuer())
}

func TestReadInputErrors(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(f *readerFixture)
		app        string
		wantReason string
		wantText   string
		wantCands  []string
	}{
		{
			name: "API not found",
			setup: func(f *readerFixture) {
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Times(2).Return(serverPages([]*managementv3.ResourceServer{}), nil)
			},
			wantReason: "api_not_found",
			wantText:   "FIX  Create it first: auth0 apis create",
		},
		{
			name: "app not found",
			setup: func(f *readerFixture) {
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{shopServer()}), nil)
				f.clients.EXPECT().Get(gomock.Any(), "Nope", gomock.Any()).Return(nil, notFound())
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPages([]*managementv3.Client{}), nil)
			},
			app:        "Nope",
			wantReason: "app_not_found",
			wantText:   "auth0 apps create",
		},
		{
			name: "two apps with the same name",
			setup: func(f *readerFixture) {
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{shopServer()}), nil)
				f.clients.EXPECT().Get(gomock.Any(), "Twin", gomock.Any()).Return(nil, notFound())
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPages([]*managementv3.Client{
					{ClientID: ptr("a"), Name: ptr("Twin")}, {ClientID: ptr("b"), Name: ptr("Twin")},
				}), nil)
			},
			app:        "Twin",
			wantReason: "ambiguous_app",
			wantText:   "Pass the client ID",
			wantCands:  []string{"a", "b"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			f.expectTenant()
			f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)
			test.setup(f)

			_, err := f.reader.Read(t.Context(), []string{shopAPI}, test.app)

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, test.wantReason, input.Reason)
			assert.Contains(t, err.Error(), test.wantText)
			assert.Equal(t, test.wantCands, input.Candidates)
		})
	}
}

func TestReadFailures(t *testing.T) {
	boom := core.NewAPIError(http.StatusForbidden, nil, errors.New("insufficient_scope"))

	tests := []struct {
		name     string
		setup    func(f *readerFixture)
		wantText string
	}{
		{
			name: "tenant settings",
			setup: func(f *readerFixture) {
				f.tenant.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, boom)
			},
			wantText: "failed to read the tenant settings",
		},
		{
			name: "custom domains",
			setup: func(f *readerFixture) {
				f.expectTenant()
				f.domains.EXPECT().List(gomock.Any()).Return(nil, boom)
			},
			wantText: "failed to list the custom domains",
		},
		{
			name: "APIs",
			setup: func(f *readerFixture) {
				f.expectTenant()
				f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, boom)
			},
			wantText: "failed to list the APIs",
		},
		{
			name: "app other than not found",
			setup: func(f *readerFixture) {
				f.expectTenant()
				f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{shopServer()}), nil)
				f.clients.EXPECT().Get(gomock.Any(), shopApp, gomock.Any()).Return(nil, boom)
			},
			wantText: "failed to read the app " + shopApp,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			test.setup(f)

			_, err := f.reader.Read(t.Context(), []string{shopAPI}, shopApp)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantText)

			var apiErr *core.APIError
			assert.ErrorAs(t, err, &apiErr, "the API error stays reachable for the scope hint")
		})
	}
}

func TestDefaultCustomDomain(t *testing.T) {
	pending := &management.CustomDomain{Domain: ptr("pending.shop.example"), Status: ptr("pending_verification"), IsDefault: ptr(true)}

	tests := []struct {
		name    string
		domains []*management.CustomDomain
		want    string
	}{
		{name: "none"},
		{name: "default is not verified", domains: []*management.CustomDomain{pending}},
		{name: "a single verified domain", domains: []*management.CustomDomain{pending, readyDomain("auth.shop.example", false)}, want: "auth.shop.example"},
		{
			name:    "several verified domains, one is the default",
			domains: []*management.CustomDomain{readyDomain("a.shop.example", false), readyDomain("b.shop.example", true)},
			want:    "b.shop.example",
		},
		{
			name:    "several verified domains, none is the default",
			domains: []*management.CustomDomain{readyDomain("a.shop.example", false), readyDomain("b.shop.example", false)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			f.domains.EXPECT().List(gomock.Any()).Return(test.domains, nil)

			got, err := f.reader.defaultCustomDomain(t.Context())

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestListPoppyAPIs(t *testing.T) {
	f := newReaderFixture(t)
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages(
		[]*managementv3.ResourceServer{
			{ID: ptr("rs_shop"), Identifier: ptr(shopAPI), Name: ptr("Shop API"), UseForPersonalAgentProtocol: ptr(true)},
			{ID: ptr("rs_off"), Identifier: ptr("https://off.example"), UseForPersonalAgentProtocol: ptr(false)},
		},
		[]*managementv3.ResourceServer{
			{ID: ptr("rs_other"), Identifier: ptr("https://other.example"), UseForPersonalAgentProtocol: ptr(true)},
			{ID: ptr("rs_unset"), Identifier: ptr("https://unset.example")},
		},
	), nil)

	got, err := f.reader.ListPoppyAPIs(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []API{
		{ID: "rs_shop", Identifier: shopAPI, Name: "Shop API", Poppy: true},
		{ID: "rs_other", Identifier: "https://other.example", Poppy: true},
	}, got)
}

func TestListPoppyAPIsFailure(t *testing.T) {
	f := newReaderFixture(t)
	boom := errors.New("boom")
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, boom)

	_, err := f.reader.ListPoppyAPIs(t.Context())

	require.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "failed to list the APIs")
}

func TestSettings(t *testing.T) {
	f := newReaderFixture(t)
	f.expectTenant()

	got, err := f.reader.Settings(t.Context())

	require.NoError(t, err)
	assert.Equal(t, Tenant{
		CIMD:                     true,
		ResourceParameterProfile: "compatibility",
		DefaultAudience:          shopAPI,
		FriendlyName:             "Shop",
		CompanyDomains:           []string{"https://shop.example"},
	}, got)
}

func TestSettingsFailure(t *testing.T) {
	f := newReaderFixture(t)
	boom := errors.New("boom")
	f.tenant.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, boom)

	_, err := f.reader.Settings(t.Context())

	require.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "failed to read the tenant settings")
}

func TestBrowserJoinApps(t *testing.T) {
	f := newReaderFixture(t)
	f.clients.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...any) (*auth0.ClientListPage, error) {
			assert.Equal(t, appFields, *request.Fields)
			assert.True(t, *request.IncludeFields)

			return clientPages(
				[]*managementv3.Client{
					{ClientID: ptr("plain"), Name: ptr("Plain")},
					{ClientID: ptr(shopApp), Name: ptr("Shop Web"), IsFirstParty: ptr(true), InitiateLoginURI: ptr("https://shop.example/login"), AllowPersonalAgentBrowserJoin: ptr(true)},
				},
				[]*managementv3.Client{{ClientID: ptr("off"), AllowPersonalAgentBrowserJoin: ptr(false)}, {ClientID: ptr("third"), AllowPersonalAgentBrowserJoin: ptr(true)}},
			), nil
		})

	got, err := f.reader.BrowserJoinApps(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []App{
		{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
		{ClientID: "third", BrowserJoin: true},
	}, got)
}

func TestBrowserJoinAppsFailures(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name  string
		setup func(f *readerFixture)
	}{
		{
			name:  "the list call",
			setup: func(f *readerFixture) { f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, boom) },
		},
		{
			name: "a later page",
			setup: func(f *readerFixture) {
				page := clientPages([]*managementv3.Client{{ClientID: ptr("a")}})
				page.NextPageFunc = func(context.Context) (*core.Page[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent], error) {
					return nil, boom
				}
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(page, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			test.setup(f)

			_, err := f.reader.BrowserJoinApps(t.Context())

			require.ErrorIs(t, err, boom)
			assert.ErrorContains(t, err, "failed to list the clients")
		})
	}
}

func TestBrowserJoinAppsOnATenantWithoutThePoppyBeta(t *testing.T) {
	body := `{"statusCode":400,"error":"Bad Request","message":"Query validation error: 'allow_personal_agent_browser_join' is not an allowed field","errorCode":"invalid_query_string"}`

	tests := []struct {
		name      string
		err       error
		wantBeta  bool
		wantInErr string
	}{
		{name: "the real error body", err: core.NewAPIError(http.StatusBadRequest, nil, errors.New(body)), wantBeta: true},
		{
			name:      "another bad request",
			err:       core.NewAPIError(http.StatusBadRequest, nil, errors.New(`{"statusCode":400,"errorCode":"invalid_body"}`)),
			wantInErr: "failed to list the clients",
		},
		{
			name:      "another status with the same code",
			err:       core.NewAPIError(http.StatusForbidden, nil, errors.New(body)),
			wantInErr: "failed to list the clients",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newReaderFixture(t)
			f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.err)

			_, err := f.reader.BrowserJoinApps(t.Context())

			var input *InputError
			if !test.wantBeta {
				require.False(t, errors.As(err, &input))
				require.ErrorIs(t, err, test.err)
				assert.ErrorContains(t, err, test.wantInErr)

				return
			}

			require.ErrorAs(t, err, &input)
			assert.Equal(t, ReasonPoppyUnavailable, input.Reason)
			assert.Equal(t, "This tenant doesn't have the Poppy beta.", input.Message)
			assert.Equal(t, "Contact Auth0 support to enable it.", input.Fix)
		})
	}
}

func TestListAPIs(t *testing.T) {
	f := newReaderFixture(t)
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages(
		[]*managementv3.ResourceServer{
			{ID: ptr("rs_shop"), Identifier: ptr(shopAPI), UseForPersonalAgentProtocol: ptr(true), AgentSubjectClaims: ptr(managementv3.ResourceServerAgentSubjectClaimsResponseEnum("auth0-v1"))},
			{ID: ptr("rs_off"), Identifier: ptr("https://off.example")},
		},
	), nil)

	got, err := f.reader.ListAPIs(t.Context())

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, shopAPI, got[0].Identifier)
	assert.True(t, got[0].Poppy)
	assert.Equal(t, "auth0-v1", got[0].AgentSubjectClaims)
	assert.False(t, got[1].Poppy, "an API with the protocol off is listed too")
}

func TestListAPIsFailure(t *testing.T) {
	f := newReaderFixture(t)
	f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	_, err := f.reader.ListAPIs(t.Context())

	assert.ErrorContains(t, err, "failed to list the APIs")
}

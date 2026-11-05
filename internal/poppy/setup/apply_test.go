package setup

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/poppy/config"
)

const loginURI = "https://shop.example/auth/login"

// applyHarness records the Management API writes of an apply, in order, and can
// fail one of them.
type applyHarness struct {
	applier *Applier
	calls   []string
	fail    map[string]error

	tenant *managementv3.UpdateTenantSettingsRequestContent
	server *managementv3.UpdateResourceServerRequestContent
	// The ID the API was updated by.
	serverID string
	client   *managementv3.UpdateClientRequestContent
	// The ID the app was updated by.
	clientID string
	grant    *managementv3.CreateClientGrantRequestContent
}

func newApplyHarness(t *testing.T) *applyHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &applyHarness{fail: map[string]error{}}

	tenant := mock.NewMockTenantSettingsAPIV3(ctrl)
	tenant.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.UpdateTenantSettingsRequestContent, _ ...option.RequestOption) (*managementv3.UpdateTenantSettingsResponseContent, error) {
			h.calls = append(h.calls, "tenant")
			h.tenant = request

			return &managementv3.UpdateTenantSettingsResponseContent{}, h.fail["tenant"]
		}).AnyTimes()

	servers := mock.NewMockResourceServerAPIV3(ctrl)
	servers.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, id string, request *managementv3.UpdateResourceServerRequestContent, _ ...option.RequestOption) (*managementv3.UpdateResourceServerResponseContent, error) {
			h.calls = append(h.calls, "api")
			h.serverID, h.server = id, request

			return &managementv3.UpdateResourceServerResponseContent{}, h.fail["api"]
		}).AnyTimes()

	clients := mock.NewMockClientAPIV3(ctrl)
	clients.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, id string, request *managementv3.UpdateClientRequestContent, _ ...option.RequestOption) (*managementv3.UpdateClientResponseContent, error) {
			h.calls = append(h.calls, "app")
			h.clientID, h.client = id, request

			return &managementv3.UpdateClientResponseContent{}, h.fail["app"]
		}).AnyTimes()

	grants := mock.NewMockClientGrantAPIV3(ctrl)
	grants.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
			h.calls = append(h.calls, "grant")
			h.grant = request

			return &managementv3.CreateClientGrantResponseContent{}, h.fail["grant"]
		}).AnyTimes()

	h.applier = &Applier{Tenant: tenant, Servers: servers, Clients: clients, Grants: grants}

	return h
}

// freshApply is a tenant that has never been set up, with inputs that change
// every kind of resource, and a new scope on top of the two the API has.
func freshApply(t *testing.T) (*State, Inputs) {
	t.Helper()

	state, in := freshState(t)
	state.Tenant.CompanyDomains = []string{"https://other.example"}
	in.LoginURI = loginURI
	in.Scopes = map[string]string{"refunds": "Return an order. Can't be undone."}
	in.Out = filepath.Join(t.TempDir(), config.DefaultFileName)

	return state, in
}

func TestApplyFreshTenant(t *testing.T) {
	state, in := freshApply(t)
	plan, err := Build(state, in)
	require.NoError(t, err)

	h := newApplyHarness(t)

	applied, err := h.applier.Apply(t.Context(), state, in, plan)

	require.NoError(t, err)
	assert.Equal(t, []string{"tenant", "api", "app", "grant"}, h.calls, "one call per change, in plan order, the file is not a call")

	descriptions := make([]string, len(plan.Changes))
	for i, change := range plan.Changes {
		descriptions[i] = change.Description
	}
	assert.Equal(t, descriptions, applied)

	assert.Equal(t, &managementv3.UpdateTenantSettingsRequestContent{
		ClientIDMetadataDocumentSupported:          managementv3.Bool(true),
		AuthorizationResponseIssParameterSupported: managementv3.Bool(true),
		ResourceParameterProfile:                   ptr(managementv3.TenantSettingsResourceParameterProfileCompatibility),
		DefaultAudience:                            managementv3.String(shopAPI),
		PersonalAgentProtocolCompanyDomains:        []string{"https://other.example", "https://shop.example"},
	}, h.tenant, "the company domain is appended to the ones the tenant lists")
	assert.Equal(t, []string{"https://other.example"}, state.Tenant.CompanyDomains, "the state is not changed")

	assert.Equal(t, "rs_shop", h.serverID)
	assert.Equal(t, &managementv3.UpdateResourceServerRequestContent{
		Scopes: []*managementv3.ResourceServerScope{
			{Value: "orders:read", Description: managementv3.String("See your orders and their status. Read only.")},
			{Value: "checkout", Description: managementv3.String("Place orders and pay with your saved card. Can't be undone.")},
			{Value: BrowserSignInScope, Description: managementv3.String(BrowserSignInDescription)},
			{Value: "refunds", Description: managementv3.String("Return an order. Can't be undone.")},
		},
		AllowOfflineAccess:          managementv3.Bool(true),
		AgentSubjectClaims:          ptr(managementv3.ResourceServerAgentSubjectClaimsSchemaEnumAuth0V1),
		UseForPersonalAgentProtocol: managementv3.Bool(true),
		SubjectTypeAuthorization: &managementv3.ResourceServerSubjectTypeAuthorization{
			AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{
				Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant),
			},
		},
	}, h.server, "the existing scopes are kept and the new one is added")

	assert.Equal(t, shopApp, h.clientID)
	assert.Equal(t, &managementv3.UpdateClientRequestContent{
		InitiateLoginURI:              managementv3.String(loginURI),
		AllowPersonalAgentBrowserJoin: managementv3.Bool(true),
	}, h.client, "no callbacks and no session transfer")

	assert.Equal(t, &managementv3.CreateClientGrantRequestContent{
		ClientID:    managementv3.String(shopApp),
		Audience:    shopAPI,
		Scope:       []string{"orders:read", "checkout", BrowserSignInScope, "refunds"},
		SubjectType: ptr(managementv3.ClientGrantSubjectTypeEnumAnonymousUser),
	}, h.grant)

	written, err := config.Read(in.Out)
	require.NoError(t, err, "the file is written, and is valid")
	assert.Equal(t, "https://"+tenantURL+"/", written.Issuer)
	assert.Contains(t, written.Scopes, "refunds")
}

func TestApplyOnlyWhatChanges(t *testing.T) {
	state, in := freshApply(t)
	state.Tenant = Tenant{
		FriendlyName:             "Shop",
		CIMD:                     true,
		ResourceParameterProfile: "compatibility",
		DefaultAudience:          "https://other.example/api",
		CompanyDomains:           []string{"https://shop.example"},
	}
	state.APIs[0].AllowOfflineAccess = true
	state.APIs[0].AgentSubjectClaims = "auth0-v1"
	state.APIs[0].Poppy = true
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, Scope{Name: BrowserSignInScope, Description: BrowserSignInDescription})
	state.HasGuestGrant = map[string]bool{shopAPI: true}
	state.App.LoginURI = loginURI
	in.Scopes = nil

	plan, err := Build(state, in)
	require.NoError(t, err)

	h := newApplyHarness(t)

	_, err = h.applier.Apply(t.Context(), state, in, plan)

	require.NoError(t, err)
	assert.Equal(t, []string{"tenant", "api", "app"}, h.calls)
	assert.Equal(t, &managementv3.UpdateTenantSettingsRequestContent{
		AuthorizationResponseIssParameterSupported: managementv3.Bool(true),
	}, h.tenant, "a single request with only the changed field")
	assert.Equal(t, &managementv3.UpdateResourceServerRequestContent{
		SubjectTypeAuthorization: &managementv3.ResourceServerSubjectTypeAuthorization{
			AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{
				Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant),
			},
		},
	}, h.server, "no scopes are sent when none changes")
	assert.Equal(t, &managementv3.UpdateClientRequestContent{AllowPersonalAgentBrowserJoin: managementv3.Bool(true)}, h.client)
}

func TestApplyKeepsTheOtherSubjectTypePolicies(t *testing.T) {
	state, in := freshApply(t)
	state.APIs[0].UserPolicy = "deny_all"
	state.APIs[0].ClientPolicy = "require_client_grant"

	plan, err := Build(state, in)
	require.NoError(t, err)

	h := newApplyHarness(t)

	_, err = h.applier.Apply(t.Context(), state, in, plan)

	require.NoError(t, err)
	assert.Equal(t, &managementv3.ResourceServerSubjectTypeAuthorization{
		User:   &managementv3.ResourceServerSubjectTypeAuthorizationUser{Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationUserPolicyEnumDenyAll)},
		Client: &managementv3.ResourceServerSubjectTypeAuthorizationClient{Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationClientPolicyEnumRequireClientGrant)},
		AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{
			Policy: ptr(managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant),
		},
	}, h.server.SubjectTypeAuthorization)
}

func TestApplyFileOnly(t *testing.T) {
	state, in := freshApply(t)
	state.File = nil
	state.Tenant = Tenant{
		CIMD: true, IssParameter: true, ResourceParameterProfile: "compatibility",
		DefaultAudience: shopAPI, FriendlyName: "Shop", CompanyDomains: []string{"https://shop.example"},
	}
	state.APIs[0] = configuredState(t).APIs[0]
	state.App = configuredState(t).App
	state.HasGuestGrant = map[string]bool{shopAPI: true}
	in.Scopes = nil
	in.LoginURI = ""

	plan, err := Build(state, in)
	require.NoError(t, err)
	require.Equal(t, []Kind{KindFile}, kinds(plan))

	h := newApplyHarness(t)

	applied, err := h.applier.Apply(t.Context(), state, in, plan)

	require.NoError(t, err)
	assert.Empty(t, h.calls, "no Management API write")
	assert.Equal(t, []string{"write " + in.Out}, applied)
	assert.FileExists(t, in.Out)
}

// TestApplyIsIdempotent is the idempotency test: the plan built from the state
// the requests leave behind is empty.
func TestApplyIsIdempotent(t *testing.T) {
	state, in := freshApply(t)
	plan, err := Build(state, in)
	require.NoError(t, err)

	h := newApplyHarness(t)
	_, err = h.applier.Apply(t.Context(), state, in, plan)
	require.NoError(t, err)

	after := *state
	after.Tenant.CIMD = *h.tenant.ClientIDMetadataDocumentSupported
	after.Tenant.IssParameter = *h.tenant.AuthorizationResponseIssParameterSupported
	after.Tenant.ResourceParameterProfile = string(*h.tenant.ResourceParameterProfile)
	after.Tenant.DefaultAudience = *h.tenant.DefaultAudience
	after.Tenant.CompanyDomains = h.tenant.PersonalAgentProtocolCompanyDomains

	api := state.APIs[0]
	api.Scopes = nil
	for _, scope := range h.server.Scopes {
		api.Scopes = append(api.Scopes, Scope{Name: scope.Value, Description: *scope.Description})
	}
	api.AllowOfflineAccess = *h.server.AllowOfflineAccess
	api.AgentSubjectClaims = string(*h.server.AgentSubjectClaims)
	api.Poppy = *h.server.UseForPersonalAgentProtocol
	api.GuestPolicy = string(*h.server.SubjectTypeAuthorization.AnonymousUser.Policy)
	after.APIs = []API{api}

	after.App.LoginURI = *h.client.InitiateLoginURI
	after.App.BrowserJoin = *h.client.AllowPersonalAgentBrowserJoin
	after.HasGuestGrant = map[string]bool{h.grant.Audience: *h.grant.SubjectType == managementv3.ClientGrantSubjectTypeEnumAnonymousUser}

	after.File, err = config.Read(in.Out)
	require.NoError(t, err)

	again, err := Build(&after, in)

	require.NoError(t, err)
	assert.True(t, again.Empty(), "a second run changes nothing: %+v", again.Changes)
}

func TestApplyFailures(t *testing.T) {
	apiErr := errors.New("boom")

	tests := []struct {
		name          string
		fail          string
		wantApplied   []string
		wantRemaining []string
		wantCalls     []string
	}{
		{name: "the tenant", fail: "tenant", wantCalls: []string{"tenant"}},
		{
			name: "the API", fail: "api", wantCalls: []string{"tenant", "api"},
			wantApplied: []string{"tenant"}, wantRemaining: []string{"app", "grant", "file"},
		},
		{
			name: "the app", fail: "app", wantCalls: []string{"tenant", "api", "app"},
			wantApplied: []string{"tenant", "api"}, wantRemaining: []string{"grant", "file"},
		},
		{
			name: "the guest grant", fail: "grant", wantCalls: []string{"tenant", "api", "app", "grant"},
			wantApplied: []string{"tenant", "api", "app"}, wantRemaining: []string{"file"},
		},
		{
			name: "the file", fail: "file", wantCalls: []string{"tenant", "api", "app", "grant"},
			wantApplied: []string{"tenant", "api", "app", "grant"}, wantRemaining: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := freshApply(t)
			plan, err := Build(state, in)
			require.NoError(t, err)

			byKind := map[string]string{}
			for _, change := range plan.Changes {
				byKind[string(change.Kind)] = change.Description
			}

			h := newApplyHarness(t)
			if test.fail == "file" {
				in.Out = filepath.Join(t.TempDir(), "missing", config.DefaultFileName)
			} else {
				h.fail[test.fail] = apiErr
			}

			applied, err := h.applier.Apply(t.Context(), state, in, plan)

			require.Error(t, err)
			assert.Nil(t, applied)
			assert.Equal(t, test.wantCalls, h.calls, "stops at the first failure")
			assert.NoFileExists(t, in.Out, "the file is written last, and not after a failure")

			if test.wantApplied == nil {
				var partial *PartialError
				assert.False(t, errors.As(err, &partial), "nothing was changed")
				assert.ErrorIs(t, err, apiErr)

				return
			}

			var partial *PartialError
			require.ErrorAs(t, err, &partial)
			assert.Equal(t, descriptionsOf(byKind, test.wantApplied), partial.Applied)
			assert.Equal(t, byKind[test.fail], partial.Failed)
			assert.Equal(t, descriptionsOf(byKind, test.wantRemaining), partial.Remaining)
			assert.Contains(t, partial.Error(), "Running the command again continues")
			assert.Contains(t, partial.Error(), "failed: "+byKind[test.fail])

			if test.fail != "file" {
				assert.ErrorIs(t, err, apiErr)
			}
		})
	}
}

func descriptionsOf(byKind map[string]string, kinds []string) []string {
	out := make([]string, len(kinds))
	for i, kind := range kinds {
		out[i] = byKind[kind]
	}

	return out
}

func TestPartialErrorDetails(t *testing.T) {
	tests := []struct {
		name string
		err  *PartialError
		want string
	}{
		{
			name: "with remaining changes",
			err:  &PartialError{Applied: []string{"a"}, Failed: "b", Remaining: []string{"c", "d"}, Err: errors.New("boom")},
			want: `{"code":"partial","applied":["a"],"failed":"b","remaining":["c","d"]}`,
		},
		{
			name: "none remaining",
			err:  &PartialError{Applied: []string{"a"}, Failed: "b", Err: errors.New("boom")},
			want: `{"code":"partial","applied":["a"],"failed":"b","remaining":[]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.JSONEq(t, test.want, string(test.err.ErrorDetails()))
			assert.True(t, json.Valid(test.err.ErrorDetails()))
		})
	}

	err := &PartialError{Applied: []string{"a"}, Failed: "b", Err: errors.New("boom")}
	assert.NotContains(t, err.Error(), "remaining", "no remaining line when nothing is left")
	assert.EqualError(t, errors.Unwrap(err), "boom")
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name        string
		yes         bool
		interactive bool
		want        Approval
	}{
		{name: "yes in a terminal", yes: true, interactive: true, want: Proceed},
		{name: "yes without a terminal", yes: true, want: Proceed},
		{name: "a terminal", interactive: true, want: Ask},
		{name: "no terminal and no yes", want: Refuse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Decide(test.yes, test.interactive))
		})
	}
}

func TestApplyStopsWhenTheFileCannotBeDerived(t *testing.T) {
	state, in := freshApply(t)
	plan, err := Build(state, in)
	require.NoError(t, err)

	in.Scopes = map[string]string{"orders:read": "A different description."}
	h := newApplyHarness(t)

	_, err = h.applier.Apply(t.Context(), state, in, plan)

	var input *InputError
	require.ErrorAs(t, err, &input)
	assert.Empty(t, h.calls, "nothing is written before the inputs are checked")

	in.Scopes = nil
	in.APIURLs = nil
	state.File = nil

	_, err = h.applier.Apply(t.Context(), state, in, plan)

	var missing *config.MissingFieldsError
	require.ErrorAs(t, err, &missing)
	assert.Empty(t, h.calls)
}

func TestApplyGuestGrantCarriesTheChosenScopes(t *testing.T) {
	state, in := freshApply(t)
	in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: {"orders:read"}}
	plan, err := Build(state, in)
	require.NoError(t, err)

	h := newApplyHarness(t)

	_, err = h.applier.Apply(t.Context(), state, in, plan)

	require.NoError(t, err)
	require.NotNil(t, h.grant)
	assert.Equal(t, []string{"orders:read"}, h.grant.Scope)
}

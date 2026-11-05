package agents

import (
	"errors"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func previewResponse(clientID string) *managementv3.PreviewCimdMetadataResponseContent {
	resp := &managementv3.PreviewCimdMetadataResponseContent{
		Validation: &managementv3.CimdValidationResult{Valid: true, Violations: []string{}, Warnings: []string{"logo is small"}},
		MappedFields: &managementv3.CimdMappedClientFields{
			Name:      ptr("Meta Muse"),
			LogoURI:   ptr("https://muse.example/logo.png"),
			Callbacks: []string{"https://muse.example/oauth/callback"},
			ClientAuthenticationMethods: &managementv3.CimdMappedClientAuthenticationMethods{
				PrivateKeyJwt: &managementv3.CimdMappedClientAuthenticationMethodsPrivateKeyJwt{
					Credentials: []*managementv3.CimdMappedPrivateKeyJwtCredential{
						{CredentialType: "public_key", Kid: "k1", Alg: "RS256"},
						{CredentialType: "public_key", Kid: "k2", Alg: "PS256"},
					},
				},
			},
		},
	}

	if clientID != "" {
		resp.ClientID = ptr(clientID)
	}

	return resp
}

func TestPreview(t *testing.T) {
	tests := []struct {
		name       string
		resp       *managementv3.PreviewCimdMetadataResponseContent
		apiErr     error
		want       *Preview
		violations []string
		wantError  string
	}{
		{
			name: "valid document",
			resp: previewResponse(""),
			want: &Preview{
				URL:          museURL,
				Name:         "Meta Muse",
				LogoURI:      "https://muse.example/logo.png",
				RedirectURLs: []string{"https://muse.example/oauth/callback"},
				Keys:         []Key{{KID: "k1", Alg: "RS256"}, {KID: "k2", Alg: "PS256"}},
				Warnings:     []string{"logo is small"},
			},
		},
		{
			name: "existing agent app",
			resp: previewResponse(museApp),
			want: &Preview{
				URL:          museURL,
				Name:         "Meta Muse",
				LogoURI:      "https://muse.example/logo.png",
				RedirectURLs: []string{"https://muse.example/oauth/callback"},
				Keys:         []Key{{KID: "k1", Alg: "RS256"}, {KID: "k2", Alg: "PS256"}},
				Warnings:     []string{"logo is small"},
				ClientID:     museApp,
			},
		},
		{
			name:       "document violations",
			resp:       &managementv3.PreviewCimdMetadataResponseContent{Validation: &managementv3.CimdValidationResult{Violations: []string{"redirect_uris is empty"}}},
			violations: []string{"redirect_uris is empty"},
		},
		{
			name:       "document could not be fetched",
			resp:       &managementv3.PreviewCimdMetadataResponseContent{Errors: []string{"unable to fetch the document"}},
			violations: []string{"unable to fetch the document"},
		},
		{
			name:       "no validation result",
			resp:       &managementv3.PreviewCimdMetadataResponseContent{},
			violations: nil,
		},
		{
			name:      "API error",
			apiErr:    errors.New("boom"),
			wantError: "failed to preview the agent metadata document: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.clients.EXPECT().
				PreviewCimdMetadata(gomock.Any(), &managementv3.PreviewCimdMetadataRequestContent{ExternalClientID: museURL}).
				Return(test.resp, test.apiErr)

			got, err := f.svc.Preview(t.Context(), museURL)

			switch {
			case test.wantError != "":
				assert.EqualError(t, err, test.wantError)
			case test.want != nil:
				require.NoError(t, err)
				assert.Equal(t, test.want, got)
			default:
				var invalid *InvalidDocumentError
				require.ErrorAs(t, err, &invalid)
				assert.Equal(t, museURL, invalid.URL)
				assert.Equal(t, test.violations, invalid.Violations)
				assert.Nil(t, got)
			}
		})
	}
}

func TestInvalidDocumentError(t *testing.T) {
	tests := []struct {
		name       string
		violations []string
		wantNote   bool
	}{
		{name: "key violation", violations: []string{"jwks_uri must serve at least one key"}, wantNote: true},
		{name: "algorithm violation", violations: []string{"unsupported alg ES256"}, wantNote: true},
		{name: "other violation", violations: []string{"redirect_uris is empty"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := &InvalidDocumentError{URL: museURL, Violations: test.violations}

			assert.Contains(t, err.Error(), "the agent metadata document at "+museURL+" is not valid:\n  "+test.violations[0])
			assert.Equal(t, test.wantNote, strings.Contains(err.Error(), ecKeysNote))
		})
	}
}

func TestChooseScopes(t *testing.T) {
	other := API{ID: "rs_two", Identifier: otherAPI, Scopes: []Scope{{Name: "orders:read"}, {Name: "refund"}}}

	tests := []struct {
		name      string
		apis      []API
		names     []string
		all       bool
		want      []Grant
		wantError string
	}{
		{
			name:  "named scopes",
			apis:  []API{shopAPIValue()},
			names: []string{"orders:read"},
			want:  []Grant{{Audience: shopAPI, Scopes: []string{"orders:read"}}},
		},
		{
			name: "all scopes",
			apis: []API{shopAPIValue(), other},
			all:  true,
			want: []Grant{
				{Audience: shopAPI, Scopes: []string{"orders:read", "checkout"}},
				{Audience: otherAPI, Scopes: []string{"orders:read", "refund"}},
			},
		},
		{
			name:      "neither named nor all",
			apis:      []API{shopAPIValue()},
			wantError: ErrScopesRequired.Error(),
		},
		{
			name:      "scope missing on one of the APIs",
			apis:      []API{shopAPIValue(), other},
			names:     []string{"checkout"},
			wantError: `scope "checkout" is not defined on ` + otherAPI,
		},
		{
			name:      "unknown scope",
			apis:      []API{shopAPIValue()},
			names:     []string{"orders:read", "nope"},
			wantError: `scope "nope" is not defined on ` + shopAPI,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ChooseScopes(test.apis, test.names, test.all)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

// registerStep sets the mock calls expected for one step of a registration.
type registerStep func(f *fixture)

func appRegistered(status error) registerStep {
	return func(f *fixture) {
		f.clients.EXPECT().
			RegisterCimdClient(gomock.Any(), &managementv3.RegisterCimdClientRequestContent{ExternalClientID: museURL}).
			Return(&managementv3.RegisterCimdClientResponseContent{ClientID: museApp}, status)
	}
}

func dpopRequired(err error) registerStep {
	return func(f *fixture) {
		f.clients.EXPECT().
			Update(gomock.Any(), museApp, &managementv3.UpdateClientRequestContent{RequireProofOfPossession: ptr(true)}).
			Return(nil, err)
	}
}

func recordCreated(record *managementv3.AgentResponseContent, err error) registerStep {
	return func(f *fixture) {
		f.agents.EXPECT().
			Create(gomock.Any(), &managementv3.CreateAgentRequestContent{Name: "Meta Muse", ClientID: ptr(museApp), ExternalAgentID: ptr(museURL)}).
			Return(record, err)
	}
}

func recordLookedUp(records ...*managementv3.AgentResponseContent) registerStep {
	return func(f *fixture) {
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(records...), nil)
	}
}

func grantChecked(audience string, existing []*managementv3.ClientGrantResponseContent, err error) registerStep {
	return func(f *fixture) {
		f.grants.EXPECT().
			List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp), Audience: ptr(audience)}).
			Return(grantPage(existing...), err)
	}
}

func grantCreated(audience string, scopes []string, err error) registerStep {
	return func(f *fixture) {
		f.grants.EXPECT().
			Create(gomock.Any(), &managementv3.CreateClientGrantRequestContent{
				ClientID:    ptr(museApp),
				Audience:    audience,
				Scope:       scopes,
				SubjectType: ptr(managementv3.ClientGrantSubjectTypeEnumUser),
			}).
			Return(&managementv3.CreateClientGrantResponseContent{ID: ptr("cgr_" + audience)}, err)
	}
}

func newRegisterRequest() RegisterRequest {
	return RegisterRequest{
		Preview: &Preview{URL: museURL, Name: "Meta Muse"},
		Grants:  []Grant{{Audience: shopAPI, Scopes: []string{"orders:read"}}},
	}
}

func TestRegister(t *testing.T) {
	record := agentRecord(museAgent, "Meta Muse", museURL)

	t.Run("creates the app, the record and the grant", func(t *testing.T) {
		f := newFixture(t)
		gomock.InOrder(
			appRegisteredCall(f),
			recordCreatedCall(f, record, nil),
			grantCheckedCall(f, shopAPI, nil, nil),
			grantCreatedCall(f, shopAPI, []string{"orders:read"}, nil),
		)

		reg, err := f.svc.Register(t.Context(), newRegisterRequest())

		require.NoError(t, err)
		assert.Equal(t, &Registration{
			URL:          museURL,
			Name:         "Meta Muse",
			ClientID:     museApp,
			AppCreated:   true,
			AgentID:      museAgent,
			AgentCreated: true,
			Grants:       []GrantResult{{Grant: Grant{ID: "cgr_" + shopAPI, Audience: shopAPI, Scopes: []string{"orders:read"}}, Created: true}},
		}, reg)
	})

	t.Run("an existing agent app is updated", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Preview.ClientID = museApp
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, nil, nil)(f)
		grantCreated(shopAPI, []string{"orders:read"}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.False(t, reg.AppCreated)
	})

	t.Run("the name flag wins over the document name", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Name = "Muse for Shop"
		req.Grants = nil
		appRegistered(nil)(f)
		f.agents.EXPECT().
			Create(gomock.Any(), &managementv3.CreateAgentRequestContent{Name: "Muse for Shop", ClientID: ptr(museApp), ExternalAgentID: ptr(museURL)}).
			Return(record, nil)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, "Muse for Shop", reg.Name)
	})

	t.Run("DPoP is required only when asked for", func(t *testing.T) {
		for _, requireDPoP := range []bool{true, false} {
			t.Run(map[bool]string{true: "on", false: "off"}[requireDPoP], func(t *testing.T) {
				f := newFixture(t)
				req := newRegisterRequest()
				req.Grants = nil
				req.RequireDPoP = requireDPoP
				appRegistered(nil)(f)
				if requireDPoP {
					dpopRequired(nil)(f)
				}
				recordCreated(record, nil)(f)

				reg, err := f.svc.Register(t.Context(), req)

				require.NoError(t, err)
				assert.Equal(t, requireDPoP, reg.DPoPRequired)
			})
		}
	})

	t.Run("an agent that is already registered is looked up", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Grants = nil
		appRegistered(nil)(f)
		recordCreated(nil, apiError(409, `{"errorCode":"agent_already_exists"}`))(f)
		recordLookedUp(agentRecord("agt_other", "Other", atlasURL), record)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, museAgent, reg.AgentID)
		assert.False(t, reg.AgentCreated)
	})

	t.Run("an existing grant is left as it is", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{userGrantOf("cgr_1", shopAPI, "checkout")}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		require.Len(t, reg.Grants, 1)
		assert.Equal(t, Grant{ID: "cgr_1", Audience: shopAPI, Scopes: []string{"checkout"}}, reg.Grants[0].Grant)
		assert.False(t, reg.Grants[0].Created)
	})

	t.Run("an existing grant with the requested scopes has no note", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{userGrantOf("cgr_1", shopAPI, "orders:read")}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, []GrantResult{{Grant: Grant{ID: "cgr_1", Audience: shopAPI, Scopes: []string{"orders:read"}}}}, reg.Grants)
	})

	t.Run("an existing grant with other scopes is reported with a FIX", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Grants[0].Scopes = []string{"orders:read", "refund"}
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{userGrantOf("cgr_1", shopAPI, "checkout", "refund")}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		require.Len(t, reg.Grants, 1)
		assert.False(t, reg.Grants[0].Created)
		assert.Equal(t, []string{"checkout", "refund"}, reg.Grants[0].Scopes)
		assert.Equal(t, "the existing grant for "+shopAPI+" was left as it is and lacks orders:read and also has checkout", reg.Grants[0].Note)
		assert.Equal(t, `auth0 api patch client-grants/cgr_1 --data '{"scope":["orders:read","refund"]}'`, reg.Grants[0].Fix)
	})

	t.Run("a grant of another subject type does not count, so a user grant is created", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{clientGrantOf("cgr_m2m", shopAPI, "orders:read")}, nil)(f)
		grantCreated(shopAPI, []string{"orders:read"}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, []GrantResult{{Grant: Grant{ID: "cgr_" + shopAPI, Audience: shopAPI, Scopes: []string{"orders:read"}}, Created: true}}, reg.Grants)
	})

	t.Run("a user grant is found after a grant of another subject type", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{
			clientGrantOf("cgr_m2m", shopAPI, "other"),
			userGrantOf("cgr_user", shopAPI, "orders:read"),
		}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, "cgr_user", reg.Grants[0].ID)
		assert.False(t, reg.Grants[0].Created)
	})

	t.Run("several APIs are granted in order", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Grants = append(req.Grants, Grant{Audience: otherAPI, Scopes: []string{"refund"}})
		gomock.InOrder(
			appRegisteredCall(f),
			recordCreatedCall(f, record, nil),
			grantCheckedCall(f, shopAPI, nil, nil),
			grantCreatedCall(f, shopAPI, []string{"orders:read"}, nil),
			grantCheckedCall(f, otherAPI, nil, nil),
			grantCreatedCall(f, otherAPI, []string{"refund"}, nil),
		)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Len(t, reg.Grants, 2)
	})
}

func appRegisteredCall(f *fixture) *gomock.Call {
	return f.clients.EXPECT().
		RegisterCimdClient(gomock.Any(), &managementv3.RegisterCimdClientRequestContent{ExternalClientID: museURL}).
		Return(&managementv3.RegisterCimdClientResponseContent{ClientID: museApp}, nil)
}

func recordCreatedCall(f *fixture, record *managementv3.AgentResponseContent, err error) *gomock.Call {
	return f.agents.EXPECT().
		Create(gomock.Any(), &managementv3.CreateAgentRequestContent{Name: "Meta Muse", ClientID: ptr(museApp), ExternalAgentID: ptr(museURL)}).
		Return(record, err)
}

func grantCheckedCall(f *fixture, audience string, existing []*managementv3.ClientGrantResponseContent, err error) *gomock.Call {
	return f.grants.EXPECT().
		List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp), Audience: ptr(audience)}).
		Return(grantPage(existing...), err)
}

func grantCreatedCall(f *fixture, audience string, scopes []string, err error) *gomock.Call {
	return f.grants.EXPECT().
		Create(gomock.Any(), &managementv3.CreateClientGrantRequestContent{
			ClientID:    ptr(museApp),
			Audience:    audience,
			Scope:       scopes,
			SubjectType: ptr(managementv3.ClientGrantSubjectTypeEnumUser),
		}).
		Return(&managementv3.CreateClientGrantResponseContent{ID: ptr("cgr_" + audience)}, err)
}

func TestRegisterStopsAtTheFirstFailure(t *testing.T) {
	boom := errors.New("boom")
	record := agentRecord(museAgent, "Meta Muse", museURL)

	tests := []struct {
		name          string
		requireDPoP   bool
		calls         []registerStep
		wantError     string
		wantApplied   []string
		wantRemaining []string
	}{
		{
			name:      "registering the app",
			calls:     []registerStep{appRegistered(boom)},
			wantError: "failed to register agent app: boom",
		},
		{
			name:          "requiring DPoP",
			requireDPoP:   true,
			calls:         []registerStep{appRegistered(nil), dpopRequired(boom)},
			wantApplied:   []string{"register agent app"},
			wantRemaining: []string{"require DPoP", "create agent record", "grant " + shopAPI},
		},
		{
			name:          "creating the record",
			calls:         []registerStep{appRegistered(nil), recordCreated(nil, boom)},
			wantApplied:   []string{"register agent app"},
			wantRemaining: []string{"create agent record", "grant " + shopAPI},
		},
		{
			name: "looking the record up after a conflict",
			calls: []registerStep{
				appRegistered(nil),
				recordCreated(nil, apiError(409, "conflict")),
				recordLookedUp(),
			},
			wantApplied:   []string{"register agent app"},
			wantRemaining: []string{"create agent record", "grant " + shopAPI},
		},
		{
			name:          "checking the grant",
			calls:         []registerStep{appRegistered(nil), recordCreated(record, nil), grantChecked(shopAPI, nil, boom)},
			wantApplied:   []string{"register agent app", "create agent record"},
			wantRemaining: []string{"grant " + shopAPI},
		},
		{
			name: "creating the grant",
			calls: []registerStep{
				appRegistered(nil), recordCreated(record, nil), grantChecked(shopAPI, nil, nil),
				grantCreated(shopAPI, []string{"orders:read"}, boom),
			},
			wantApplied:   []string{"register agent app", "create agent record"},
			wantRemaining: []string{"grant " + shopAPI},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			for _, call := range test.calls {
				call(f)
			}

			req := newRegisterRequest()
			req.RequireDPoP = test.requireDPoP

			reg, err := f.svc.Register(t.Context(), req)

			require.Error(t, err)
			assert.Nil(t, reg)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			var partial *PartialError
			require.ErrorAs(t, err, &partial)
			assert.Equal(t, test.wantApplied, partial.Applied)
			assert.Equal(t, test.wantRemaining, partial.Remaining)
		})
	}
}

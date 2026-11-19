package agents

import (
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRegisterSteps(t *testing.T) {
	boom := errors.New("boom")
	preview := &Preview{URL: museURL, Name: "Meta Muse"}

	createRecord := func(f *fixture, record *managementv3.AgentResponseContent, err error) {
		f.agents.EXPECT().
			Create(gomock.Any(), &managementv3.CreateAgentRequestContent{Name: "Meta Muse", ClientID: ptr(museApp), ExternalAgentID: ptr(museURL)}).
			Return(record, err)
	}

	t.Run("registers the app, creates the record and reads the app back", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, agentRecord(museAgent, "Meta Muse", museURL), nil)
		appReadCall(f, agentApp(museApp, museURL, museAgent))

		reg := newRegistration(preview, "")
		steps := f.svc.registerSteps(reg, false)

		assert.Equal(t, []string{"register agent app", "create agent record", "read agent app"}, stepStrings(steps))
		require.NoError(t, RunSteps(t.Context(), steps))
		assert.Equal(t, &Registration{URL: museURL, Name: "Meta Muse", ClientID: museApp, AppCreated: true, AgentID: museAgent, AgentCreated: true}, reg)
	})

	t.Run("requires DPoP between the app and the record", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		dpopRequired(nil)(f)
		createRecord(f, agentRecord(museAgent, "Meta Muse", museURL), nil)
		appReadCall(f, withDPoP(agentApp(museApp, museURL, museAgent), true))

		reg := newRegistration(preview, "")
		steps := f.svc.registerSteps(reg, true)

		assert.Equal(t, []string{"register agent app", "require DPoP", "create agent record", "read agent app"}, stepStrings(steps))
		require.NoError(t, RunSteps(t.Context(), steps))
		assert.True(t, reg.DPoPRequired)
	})

	t.Run("an existing agent record is found, and the app is already linked to it", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, nil, core409())
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(agentRecord(museAgent, "Meta Muse", museURL)), nil)
		appReadCall(f, agentApp(museApp, museURL, museAgent))

		reg := newRegistration(&Preview{URL: museURL, Name: "Meta Muse", ClientID: museApp}, "")
		require.NoError(t, RunSteps(t.Context(), f.svc.registerSteps(reg, false)))

		assert.False(t, reg.AppCreated)
		assert.False(t, reg.AgentCreated)
		assert.Equal(t, museAgent, reg.AgentID)
	})

	t.Run("an existing agent record that the app is not linked to is linked to it", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, nil, core409())
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(agentRecord(museAgent, "Meta Muse", museURL)), nil)
		appReadCall(f, agentApp(museApp, museURL, ""))
		f.clients.EXPECT().
			Update(gomock.Any(), museApp, &managementv3.UpdateClientRequestContent{AgentID: ptr(museAgent)}).
			Return(nil, nil)

		reg := newRegistration(preview, "")

		require.NoError(t, RunSteps(t.Context(), f.svc.registerSteps(reg, false)))
		assert.Equal(t, museAgent, reg.AgentID)
	})

	t.Run("an existing agent record that the app is linked away from is reported, and nothing is linked", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, nil, core409())
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(agentRecord(museAgent, "Meta Muse", museURL)), nil)
		appReadCall(f, agentApp(museApp, museURL, "agt_other"))

		err := RunSteps(t.Context(), f.svc.registerSteps(newRegistration(preview, ""), false))

		var mismatch *LinkMismatchError
		require.ErrorAs(t, err, &mismatch)
		assert.Equal(t, &LinkMismatchError{URL: museURL, ClientID: museApp, LinkedAgentID: "agt_other", RecordID: museAgent}, mismatch)
		assert.ErrorContains(t, err, "the agent app "+museApp+" is linked to agent agt_other, but the agent record for "+museURL+" is "+museAgent)
	})

	t.Run("a record that was created is not checked for its link", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, agentRecord(museAgent, "Meta Muse", museURL), nil)
		// The server linked it on creation, so the app listed may be stale.
		appReadCall(f, agentApp(museApp, museURL, ""))

		require.NoError(t, RunSteps(t.Context(), f.svc.registerSteps(newRegistration(preview, ""), false)))
	})

	t.Run("linking the app fails", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, nil, core409())
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(agentRecord(museAgent, "Meta Muse", museURL)), nil)
		appReadCall(f, agentApp(museApp, museURL, ""))
		f.clients.EXPECT().Update(gomock.Any(), museApp, gomock.Any()).Return(nil, boom)

		err := RunSteps(t.Context(), f.svc.registerSteps(newRegistration(preview, ""), false))

		assert.ErrorIs(t, err, boom)
	})

	t.Run("no app is found when it is read back", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, agentRecord(museAgent, "Meta Muse", museURL), nil)
		appReadCall(f)

		err := RunSteps(t.Context(), f.svc.registerSteps(newRegistration(preview, ""), false))

		assert.ErrorContains(t, err, "no agent app was found for "+museURL)
	})

	t.Run("a failing record is a partial failure", func(t *testing.T) {
		f := newFixture(t)
		appRegisteredCall(f)
		createRecord(f, nil, boom)

		err := RunSteps(t.Context(), f.svc.registerSteps(newRegistration(preview, ""), false))

		var partial *PartialError
		require.ErrorAs(t, err, &partial)
		assert.Equal(t, []string{"register agent app"}, partial.Applied)
		assert.Equal(t, "create agent record", partial.Failed)
		assert.Equal(t, []string{"read agent app"}, partial.Remaining)
	})

	t.Run("the name defaults to the document's", func(t *testing.T) {
		assert.Equal(t, "Chosen", newRegistration(preview, "Chosen").Name)
		assert.Equal(t, "Meta Muse", newRegistration(preview, "").Name)
	})
}

func TestChooseScopes(t *testing.T) {
	other := API{ID: "rs_two", Identifier: otherAPI, Scopes: []Scope{{Name: "orders:read"}, {Name: "refund"}}}
	disjoint := API{ID: "rs_three", Identifier: "https://crm.example/api", Scopes: []Scope{{Name: "contacts:read"}}}

	tests := []struct {
		name      string
		apis      []API
		names     []string
		all       bool
		want      []Grant
		wantError string
		// The type of the error, and what it holds.
		wantUnknown  *UnknownScopeError
		wantNoScopes string
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
			name:      "names that are blank are not names",
			apis:      []API{shopAPIValue()},
			names:     []string{"", " "},
			wantError: ErrScopesRequired.Error(),
		},
		{
			name:  "two APIs with disjoint scopes each get the scopes they offer",
			apis:  []API{shopAPIValue(), disjoint},
			names: []string{"orders:read", "contacts:read"},
			want: []Grant{
				{Audience: shopAPI, Scopes: []string{"orders:read"}},
				{Audience: "https://crm.example/api", Scopes: []string{"contacts:read"}},
			},
		},
		{
			name:  "a name that several APIs offer is granted on each of them",
			apis:  []API{shopAPIValue(), other},
			names: []string{"orders:read", "refund"},
			want: []Grant{
				{Audience: shopAPI, Scopes: []string{"orders:read"}},
				{Audience: otherAPI, Scopes: []string{"orders:read", "refund"}},
			},
		},
		{
			name:  "a scope pinned to an API by identifier is granted on that API only",
			apis:  []API{shopAPIValue(), other},
			names: []string{shopAPI + "=orders:read", "refund"},
			want: []Grant{
				{Audience: shopAPI, Scopes: []string{"orders:read"}},
				{Audience: otherAPI, Scopes: []string{"refund"}},
			},
		},
		{
			name:  "a scope pinned to an API by ID",
			apis:  []API{shopAPIValue(), other},
			names: []string{"rs_two=orders:read", "checkout"},
			want: []Grant{
				{Audience: shopAPI, Scopes: []string{"checkout"}},
				{Audience: otherAPI, Scopes: []string{"orders:read"}},
			},
		},
		{
			name:        "a scope pinned to an API that does not offer it",
			apis:        []API{shopAPIValue(), other},
			names:       []string{otherAPI + "=checkout"},
			wantUnknown: &UnknownScopeError{Scope: "checkout", APIs: []string{otherAPI}},
			wantError:   `scope "checkout" is not defined on ` + otherAPI,
		},
		{
			name:  "a name with an equals sign that is not an API is a scope name",
			apis:  []API{{ID: "rs_eq", Identifier: shopAPI, Scopes: []Scope{{Name: "a=b"}}}},
			names: []string{"a=b"},
			want:  []Grant{{Audience: shopAPI, Scopes: []string{"a=b"}}},
		},
		{
			name:        "a name that no API offers",
			apis:        []API{shopAPIValue(), other},
			names:       []string{"orders:read", "nope"},
			wantUnknown: &UnknownScopeError{Scope: "nope", APIs: []string{shopAPI, otherAPI}},
			wantError:   `scope "nope" is not defined on ` + shopAPI + ", " + otherAPI,
		},
		{
			name:        "an unknown name with one API",
			apis:        []API{shopAPIValue()},
			names:       []string{"orders:read", "nope"},
			wantUnknown: &UnknownScopeError{Scope: "nope", APIs: []string{shopAPI}},
			wantError:   `scope "nope" is not defined on ` + shopAPI,
		},
		{
			name:         "an API left with no scope",
			apis:         []API{shopAPIValue(), disjoint},
			names:        []string{"orders:read"},
			wantNoScopes: "https://crm.example/api",
			wantError:    "no scope is chosen for https://crm.example/api",
		},
		{
			name:         "an API that offers no scope has none to grant with all",
			apis:         []API{{ID: "rs_empty", Identifier: "https://empty.example"}},
			all:          true,
			wantNoScopes: "https://empty.example",
			wantError:    "no scope is chosen for https://empty.example",
		},
		{
			name:  "a name given twice is granted once",
			apis:  []API{shopAPIValue()},
			names: []string{"orders:read", "orders:read", "checkout", "orders:read"},
			want:  []Grant{{Audience: shopAPI, Scopes: []string{"orders:read", "checkout"}}},
		},
		{
			name:  "a pinned and an unpinned name for the same scope are granted once",
			apis:  []API{shopAPIValue()},
			names: []string{shopAPI + "=orders:read", "orders:read"},
			want:  []Grant{{Audience: shopAPI, Scopes: []string{"orders:read"}}},
		},
		{
			name:  "all with names grants every scope once",
			apis:  []API{shopAPIValue()},
			names: []string{"checkout"},
			all:   true,
			want:  []Grant{{Audience: shopAPI, Scopes: []string{"orders:read", "checkout"}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ChooseScopes(test.apis, test.names, test.all)

			switch {
			case test.wantUnknown != nil:
				var unknown *UnknownScopeError
				require.ErrorAs(t, err, &unknown)
				assert.Equal(t, test.wantUnknown, unknown)
				assert.EqualError(t, err, test.wantError)
			case test.wantNoScopes != "":
				var none *NoScopesError
				require.ErrorAs(t, err, &none)
				assert.Equal(t, test.wantNoScopes, none.API.Identifier)
				assert.EqualError(t, err, test.wantError)
			case test.wantError != "":
				assert.EqualError(t, err, test.wantError)
			default:
				require.NoError(t, err)
				assert.Equal(t, test.want, got)
			}
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

// appRead is the call that reads the agent app back, which finds app, or no app
// when it is nil.
func appRead(app *managementv3.Client, err error) registerStep {
	return func(f *fixture) {
		if app == nil {
			f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(), err)

			return
		}

		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(app), err)
	}
}

func appReadCall(f *fixture, apps ...*managementv3.Client) *gomock.Call {
	return f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(apps...), nil)
}

func withDPoP(app *managementv3.Client, required bool) *managementv3.Client {
	app.RequireProofOfPossession = ptr(required)

	return app
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
			appReadCall(f, agentApp(museApp, museURL, museAgent)),
			grantCheckedCall(f, shopAPI, nil, nil),
			grantCreatedCall(f, shopAPI, []string{"orders:read"}, nil),
		)

		reg, err := f.svc.Register(t.Context(), newRegisterRequest())

		require.NoError(t, err)
		assert.Equal(t, &Registered{
			Registration: Registration{
				URL:          museURL,
				Name:         "Meta Muse",
				ClientID:     museApp,
				AppCreated:   true,
				AgentID:      museAgent,
				AgentCreated: true,
			},
			Grants: []GrantResult{{Grant: Grant{ID: "cgr_" + shopAPI, Audience: shopAPI, Scopes: []string{"orders:read"}}, Created: true}},
		}, reg)
	})

	t.Run("an existing agent app is updated", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		req.Preview.ClientID = museApp
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
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
		appReadCall(f, agentApp(museApp, museURL, museAgent))

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, "Muse for Shop", reg.Name)
	})

	t.Run("DPoP is reported from the state of the app", func(t *testing.T) {
		tests := []struct {
			name        string
			requireDPoP bool
			appRequires bool
			want        bool
		}{
			{name: "asked for, and the app requires it", requireDPoP: true, appRequires: true, want: true},
			{name: "not asked for, and the app does not require it", want: false},
			{name: "not asked for, but the app already requires it", appRequires: true, want: true},
			{name: "asked for, but the app does not report it", requireDPoP: true, want: false},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f := newFixture(t)
				req := newRegisterRequest()
				req.Grants = nil
				req.RequireDPoP = test.requireDPoP
				appRegistered(nil)(f)
				if test.requireDPoP {
					dpopRequired(nil)(f)
				}
				recordCreated(record, nil)(f)
				appRead(withDPoP(agentApp(museApp, museURL, museAgent), test.appRequires), nil)(f)

				reg, err := f.svc.Register(t.Context(), req)

				require.NoError(t, err)
				assert.Equal(t, test.want, reg.DPoPRequired)
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
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)

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
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
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
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
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
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
		grantChecked(shopAPI, []*managementv3.ClientGrantResponseContent{userGrantOf("cgr_1", shopAPI, "checkout", "refund")}, nil)(f)

		reg, err := f.svc.Register(t.Context(), req)

		require.NoError(t, err)
		require.Len(t, reg.Grants, 1)
		assert.False(t, reg.Grants[0].Created)
		assert.Equal(t, []string{"checkout", "refund"}, reg.Grants[0].Scopes)
		assert.Equal(t, "the existing grant for "+shopAPI+" was left as it is and lacks orders:read and also has checkout", reg.Grants[0].Note)
		assert.Equal(t, "auth0 client-grants update cgr_1 --scopes orders:read,refund", reg.Grants[0].Fix)
	})

	t.Run("a grant of another subject type does not count, so a user grant is created", func(t *testing.T) {
		f := newFixture(t)
		req := newRegisterRequest()
		appRegistered(nil)(f)
		recordCreated(record, nil)(f)
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
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
		appRead(agentApp(museApp, museURL, museAgent), nil)(f)
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
			appReadCall(f, agentApp(museApp, museURL, museAgent)),
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
		wantFailed    string
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
			wantFailed:    "require DPoP",
			wantRemaining: []string{"create agent record", "read agent app", "grant " + shopAPI},
		},
		{
			name:          "creating the record",
			calls:         []registerStep{appRegistered(nil), recordCreated(nil, boom)},
			wantApplied:   []string{"register agent app"},
			wantFailed:    "create agent record",
			wantRemaining: []string{"read agent app", "grant " + shopAPI},
		},
		{
			name: "looking the record up after a conflict",
			calls: []registerStep{
				appRegistered(nil),
				recordCreated(nil, apiError(409, "conflict")),
				recordLookedUp(),
			},
			wantApplied:   []string{"register agent app"},
			wantFailed:    "create agent record",
			wantRemaining: []string{"read agent app", "grant " + shopAPI},
		},
		{
			name:          "reading the app back",
			calls:         []registerStep{appRegistered(nil), recordCreated(record, nil), appRead(nil, boom)},
			wantApplied:   []string{"register agent app", "create agent record"},
			wantFailed:    "read agent app",
			wantRemaining: []string{"grant " + shopAPI},
		},
		{
			name: "checking the grant",
			calls: []registerStep{
				appRegistered(nil), recordCreated(record, nil), appRead(agentApp(museApp, museURL, museAgent), nil),
				grantChecked(shopAPI, nil, boom),
			},
			wantApplied:   []string{"register agent app", "create agent record", "read agent app"},
			wantFailed:    "grant " + shopAPI,
			wantRemaining: []string{},
		},
		{
			name: "creating the grant",
			calls: []registerStep{
				appRegistered(nil), recordCreated(record, nil), appRead(agentApp(museApp, museURL, museAgent), nil),
				grantChecked(shopAPI, nil, nil), grantCreated(shopAPI, []string{"orders:read"}, boom),
			},
			wantApplied:   []string{"register agent app", "create agent record", "read agent app"},
			wantFailed:    "grant " + shopAPI,
			wantRemaining: []string{},
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
			assert.Equal(t, test.wantFailed, partial.Failed)
			assert.Equal(t, test.wantRemaining, partial.Remaining)
		})
	}
}

func TestScopeDifference(t *testing.T) {
	tests := []struct {
		name      string
		existing  []string
		requested []string
		tenant    string
		wantNote  string
		wantFix   string
	}{
		{name: "equal sets", existing: []string{"b", "a"}, requested: []string{"a", "b"}},
		{
			name:      "a scope is missing",
			existing:  []string{"orders:read"},
			requested: []string{"orders:read", "checkout"},
			wantNote:  "the existing grant for " + shopAPI + " was left as it is and lacks checkout",
			wantFix:   "auth0 client-grants update cgr_1 --scopes orders:read,checkout",
		},
		{
			name:      "the tenant is repeated",
			existing:  []string{"orders:read"},
			requested: []string{"checkout"},
			tenant:    "dev.us.auth0.com",
			wantNote:  "the existing grant for " + shopAPI + " was left as it is and lacks checkout and also has orders:read",
			wantFix:   "auth0 client-grants update cgr_1 --scopes checkout --tenant dev.us.auth0.com",
		},
		{
			name:      "no scope is requested",
			existing:  []string{"orders:read"},
			requested: nil,
			wantNote:  "the existing grant for " + shopAPI + " was left as it is and also has orders:read",
			wantFix:   "auth0 client-grants update cgr_1 --no-scopes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			note, fix := scopeDifference(Grant{ID: "cgr_1", Audience: shopAPI, Scopes: test.existing}, test.requested, test.tenant)

			assert.Equal(t, test.wantNote, note)
			assert.Equal(t, test.wantFix, fix)
		})
	}
}

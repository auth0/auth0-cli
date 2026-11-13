package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// unk lists the unknown fields of an item. The list is never nil.
func unk(names ...string) []string { return append([]string{}, names...) }

func TestExtras(t *testing.T) {
	boom := errors.New("boom")
	muse := agentRecord(museAgent, "Meta Muse", museURL)
	atlas := agentRecord("agt_atlas", "Atlas", atlasURL)
	shopGrant := userGrantOf("cgr_shop", shopAPI, "orders:read", "checkout")
	shopGranted := Grant{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read", "checkout"}, SubjectType: "user"}

	tests := []struct {
		name       string
		records    []*managementv3.AgentResponseContent
		narrowed   bool
		filterErr  error
		clientsErr error
		apps       []*managementv3.Client
		grants     []*managementv3.ClientGrantResponseContent
		grantErr   error
		want       []Extras
		wantErr    error
	}{
		{
			name:    "an agent with an app reports the user grants on every API",
			records: []*managementv3.AgentResponseContent{muse},
			apps:    []*managementv3.Client{agentApp(museApp, museURL, museAgent)},
			grants:  []*managementv3.ClientGrantResponseContent{shopGrant, userGrantOf("cgr_other", otherAPI, "refund")},
			want: []Extras{{
				ClientID:       known(museApp),
				MetadataStatus: known("available"),
				DPoPRequired:   known(false),
				APIs: known([]Grant{
					shopGranted,
					{ID: "cgr_other", Audience: otherAPI, Scopes: []string{"refund"}, SubjectType: "user"},
				}),
				UnknownFields: unk(),
			}},
		},
		{
			name:    "a grant that is not for users is not an API of the agent",
			records: []*managementv3.AgentResponseContent{muse},
			apps:    []*managementv3.Client{agentApp(museApp, museURL, museAgent)},
			grants:  []*managementv3.ClientGrantResponseContent{shopGrant, clientGrantOf("cgr_m2m", otherAPI, "refund"), grantOf("cgr_none", otherAPI, "refund")},
			want: []Extras{{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{shopGranted}), UnknownFields: unk(),
			}},
		},
		{
			name:     "narrowed, only the grants on the APIs the filter covers",
			records:  []*managementv3.AgentResponseContent{muse},
			narrowed: true,
			apps:     []*managementv3.Client{agentApp(museApp, museURL, museAgent)},
			grants:   []*managementv3.ClientGrantResponseContent{shopGrant, userGrantOf("cgr_other", otherAPI, "refund")},
			want: []Extras{{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{shopGranted}), UnknownFields: unk(),
			}},
		},
		{
			name:    "in the order of the records, each with the grants of its own app",
			records: []*managementv3.AgentResponseContent{atlas, muse},
			apps:    []*managementv3.Client{agentApp(museApp, museURL, museAgent), agentApp("client_atlas", atlasURL, "agt_atlas"), agentApp("client_orphan", "https://orphan.example", "agt_orphan")},
			grants: []*managementv3.ClientGrantResponseContent{
				grantFor(museApp, userGrantOf("cgr_shop", shopAPI, "orders:read", "checkout")),
				grantFor("client_orphan", userGrantOf("cgr_orphan", otherAPI, "refund")),
			},
			want: []Extras{
				{ClientID: known("client_atlas"), MetadataStatus: known("available"), DPoPRequired: known(false), APIs: known([]Grant{}), UnknownFields: unk()},
				{ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false), APIs: known([]Grant{shopGranted}), UnknownFields: unk()},
			},
		},
		{
			name:    "an app linked to the record is read whatever its type",
			records: []*managementv3.AgentResponseContent{atlas, muse},
			apps: []*managementv3.Client{
				{ClientID: ptr("client_m2m"), AgentID: ptr("agt_atlas"), RequireProofOfPossession: ptr(true)},
				agentApp(museApp, museURL, museAgent),
			},
			grants: []*managementv3.ClientGrantResponseContent{grantFor("client_m2m", shopGrant)},
			want: []Extras{
				{
					ClientID: known("client_m2m"), MetadataStatus: known("-"), DPoPRequired: known(true), NotAgentApp: true,
					APIs: known([]Grant{shopGranted}), UnknownFields: unk(),
				},
				{ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false), APIs: known([]Grant{}), UnknownFields: unk()},
			},
		},
		{
			name:    "a record without an app has none and no grants, and DPoP does not apply",
			records: []*managementv3.AgentResponseContent{muse},
			want:    []Extras{{ClientID: known(""), MetadataStatus: known("none"), APIs: known([]Grant{}), UnknownFields: unk()}},
		},
		{
			name:       "the apps cannot be listed",
			records:    []*managementv3.AgentResponseContent{muse},
			clientsErr: boom,
			want:       []Extras{{UnknownFields: unk("client_id", "metadata_status", "dpop_required", "apis")}},
			wantErr:    boom,
		},
		{
			name:      "the audiences to narrow to cannot be read",
			records:   []*managementv3.AgentResponseContent{muse},
			narrowed:  true,
			filterErr: boom,
			apps:      []*managementv3.Client{agentApp(museApp, museURL, museAgent)},
			want: []Extras{{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false), UnknownFields: unk("apis"),
			}},
			wantErr: boom,
		},
		{
			name:     "the grants cannot be read",
			records:  []*managementv3.AgentResponseContent{muse},
			apps:     []*managementv3.Client{agentApp(museApp, museURL, museAgent)},
			grantErr: boom,
			want: []Extras{{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false), UnknownFields: unk("apis"),
			}},
			wantErr: boom,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			// The resource servers are never read here: the filter is the caller's.
			var only AudienceFilter
			filterCalls := 0
			if test.narrowed {
				only = func(context.Context) (map[string]bool, error) {
					filterCalls++

					return map[string]bool{shopAPI: true}, test.filterErr
				}
			}

			if test.clientsErr != nil {
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.clientsErr).Times(1)
			} else {
				f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(test.apps...), nil).Times(1)
			}

			// One read of the grants of the tenant, whatever the number of agents, and
			// none when no agent has an app or the filter failed.
			hasApp := false
			for _, app := range test.apps {
				hasApp = hasApp || linked(test.records, app)
			}

			if hasApp && test.filterErr == nil {
				f.grants.EXPECT().
					List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{}).
					Return(grantPage(test.grants...), test.grantErr).
					Times(1)
			}

			got, err := f.svc.Extras(t.Context(), test.records, only)

			assert.Equal(t, test.want, got)
			assert.Equal(t, map[bool]int{true: 1, false: 0}[test.narrowed], filterCalls, "the filter is called once")

			if test.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, test.wantErr)
			}
		})
	}

	t.Run("the grants of many agents are listed once", func(t *testing.T) {
		f := newFixture(t)
		records := make([]*managementv3.AgentResponseContent, 5)
		apps := make([]*managementv3.Client, 5)

		for i := range records {
			id := fmt.Sprintf("agt_%d", i)
			records[i], apps[i] = agentRecord(id, id, atlasURL), agentApp(fmt.Sprintf("client_%d", i), atlasURL, id)
		}

		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(apps...), nil).Times(1)
		f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{}).Return(grantPage(), nil).Times(1)

		got, err := f.svc.Extras(t.Context(), records, nil)

		require.NoError(t, err)
		assert.Len(t, got, 5)
	})

	t.Run("no record reads nothing", func(t *testing.T) {
		f := newFixture(t)

		got, err := f.svc.Extras(t.Context(), nil, nil)

		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("APIs that are not set leave everything unknown", func(t *testing.T) {
		svc := &Service{}

		got, err := svc.Extras(t.Context(), []*managementv3.AgentResponseContent{muse}, nil)

		assert.Error(t, err)
		assert.Equal(t, []Extras{{UnknownFields: unk("client_id", "metadata_status", "dpop_required", "apis")}}, got)
	})
}

func linked(records []*managementv3.AgentResponseContent, app *managementv3.Client) bool {
	for _, record := range records {
		if record.GetAgentID() == app.GetAgentID() {
			return true
		}
	}

	return false
}

func TestExtrasJSON(t *testing.T) {
	t.Run("an unknown value is null and named", func(t *testing.T) {
		x := Extras{ClientID: known("client_muse"), APIs: known([]Grant{{Audience: shopAPI, Scopes: []string{"a"}}})}
		x.UnknownFields = x.unknownFields()

		got, err := jsonOf(x)

		require.NoError(t, err)
		assert.JSONEq(t, `{"client_id":"client_muse","metadata_status":null,"dpop_required":null,"apis":[{"audience":"`+shopAPI+`","scopes":["a"]}],"unknown_fields":["metadata_status","dpop_required"]}`, got)
	})

	t.Run("nothing is read", func(t *testing.T) {
		x := Extras{}
		x.UnknownFields = x.unknownFields()

		got, err := jsonOf(x)

		require.NoError(t, err)
		assert.JSONEq(t, `{"client_id":null,"metadata_status":null,"dpop_required":null,"apis":null,"unknown_fields":["client_id","metadata_status","dpop_required","apis"]}`, got)
	})

	t.Run("everything is known", func(t *testing.T) {
		got, err := jsonOf(Extras{ClientID: known(""), MetadataStatus: known("none"), APIs: known([]Grant{}), UnknownFields: unk()})

		require.NoError(t, err)
		assert.JSONEq(t, `{"client_id":"","metadata_status":"none","dpop_required":null,"apis":[],"unknown_fields":[]}`, got)
	})
}

func TestLookupApp(t *testing.T) {
	t.Run("the app linked to an agent ID", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museAgent, agentApp(museApp, museURL, museAgent))

		app, err := f.svc.LookupApp(t.Context(), museAgent)

		require.NoError(t, err)
		assert.Equal(t, museApp, app.GetClientID())
	})

	t.Run("none", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museAgent)

		app, err := f.svc.LookupApp(t.Context(), museAgent)

		require.NoError(t, err)
		assert.Nil(t, app)
	})

	t.Run("the lookup fails", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		_, err := f.svc.LookupApp(t.Context(), museAgent)

		assert.ErrorContains(t, err, "boom")
	})

	t.Run("APIs that are not set", func(t *testing.T) {
		_, err := (&Service{}).LookupApp(t.Context(), museAgent)

		assert.Error(t, err)
	})
}

func TestInspect(t *testing.T) {
	boom := errors.New("boom")
	muse := agentRecord(museAgent, "Meta Muse", museURL)
	app := agentApp(museApp, museURL, museAgent)

	tests := []struct {
		name         string
		app          *managementv3.Client
		lookupErr    error
		narrowed     bool
		filterErr    error
		otherGrant   bool
		grantErr     error
		preview      *managementv3.PreviewCimdMetadataResponseContent
		previewErr   error
		credentials  []*managementv3.ClientCredential
		credErr      error
		want         Extras
		wantValid    Maybe[bool]
		wantKeys     Maybe[bool]
		wantUnknown  []string
		wantReadErr  bool
		wantNotes    []string
		wantNoteText string
	}{
		{
			name:        "everything is read",
			app:         app,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
			wantValid: known(true),
			wantKeys:  known(true),
		},
		{
			name:        "the keys changed",
			app:         app,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1")},
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
			wantValid: known(true),
			wantKeys:  known(false),
		},
		{
			name:         "the app could not be looked up",
			wantUnknown:  []string{"client_id", "metadata_status", "dpop_required", "apis", "metadata_check.valid", "keys_current"},
			wantReadErr:  true,
			lookupErr:    boom,
			wantNoteText: "The agent app could not be read, so the metadata document was not checked: boom",
		},
		{
			name: "the record has no app",
			want: Extras{ClientID: known(""), MetadataStatus: known("none"), APIs: known([]Grant{})},
		},
		{
			name:        "the document check fails",
			wantUnknown: []string{"metadata_check.valid", "keys_current"},
			wantReadErr: true,
			app:         app,
			previewErr:  boom,
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
			wantNoteText: "The metadata document could not be checked: failed to preview the agent metadata document: boom",
		},
		{
			name:        "the stored keys cannot be read",
			wantUnknown: []string{"keys_current"},
			wantReadErr: true,
			app:         app,
			preview:     previewResponse(museApp),
			credErr:     boom,
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
			wantValid:    known(true),
			wantNoteText: "The keys Auth0 stored could not be read, so they were not compared: boom",
		},
		{
			name:        "narrowed, the grants on other APIs are left out",
			app:         app,
			narrowed:    true,
			otherGrant:  true,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
			wantValid: known(true),
			wantKeys:  known(true),
		},
		{
			name:        "not narrowed, the grants on every API are reported",
			app:         app,
			otherGrant:  true,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			want: Extras{
				ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false),
				APIs: known([]Grant{
					{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"},
					{ID: "cgr_other", Audience: otherAPI, Scopes: []string{"refund"}, SubjectType: "user"},
				}),
			},
			wantValid: known(true),
			wantKeys:  known(true),
		},
		{
			name:        "the audiences to narrow to cannot be read",
			wantUnknown: []string{"apis"},
			wantReadErr: true,
			app:         app,
			narrowed:    true,
			filterErr:   boom,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			want:        Extras{ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false)},
			wantValid:   known(true),
			wantKeys:    known(true),
		},
		{
			name:        "the grants cannot be read",
			wantUnknown: []string{"apis"},
			wantReadErr: true,
			app:         app,
			grantErr:    boom,
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			want:        Extras{ClientID: known(museApp), MetadataStatus: known("available"), DPoPRequired: known(false)},
			wantValid:   known(true),
			wantKeys:    known(true),
		},
		{
			name: "a linked app that is not an agent app is not checked",
			app:  &managementv3.Client{ClientID: ptr("client_plain"), AgentID: ptr(museAgent), RequireProofOfPossession: ptr(true)},
			want: Extras{
				ClientID: known("client_plain"), MetadataStatus: known("-"), DPoPRequired: known(true), NotAgentApp: true,
				APIs: known([]Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			isAgentApp := test.lookupErr == nil && test.app != nil && test.app.GetExternalMetadataType() == managementv3.ClientExternalMetadataTypeEnumCimd

			var only AudienceFilter
			if test.narrowed {
				only = func(context.Context) (map[string]bool, error) {
					return map[string]bool{shopAPI: true}, test.filterErr
				}
			}

			if test.lookupErr == nil && test.app != nil && test.filterErr == nil {
				grants := []*managementv3.ClientGrantResponseContent{grantFor(test.app.GetClientID(), userGrantOf("cgr_shop", shopAPI, "orders:read"))}
				if test.otherGrant {
					grants = append(grants, grantFor(test.app.GetClientID(), userGrantOf("cgr_other", otherAPI, "refund")))
				}

				f.grants.EXPECT().
					List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: test.app.ClientID}).
					Return(grantPage(grants...), test.grantErr)
			}

			if isAgentApp {
				f.clients.EXPECT().
					PreviewCimdMetadata(gomock.Any(), &managementv3.PreviewCimdMetadataRequestContent{ExternalClientID: museURL}).
					Return(test.preview, test.previewErr)

				if test.preview != nil {
					f.credentials.EXPECT().List(gomock.Any(), museApp).Return(test.credentials, test.credErr)
				}
			}

			got := f.svc.Inspect(t.Context(), muse, test.app, test.lookupErr, only)

			want := test.want
			want.UnknownFields = unk(test.wantUnknown...)

			assert.Equal(t, want, got.Extras)
			assert.Equal(t, test.wantReadErr, len(got.ReadErrors) > 0, "%v", got.ReadErrors)
			assert.Equal(t, test.wantValid, got.MetadataCheck.Valid)
			assert.Equal(t, test.wantKeys, got.KeysCurrent)

			if test.wantNoteText != "" {
				assert.Equal(t, []string{test.wantNoteText}, got.Notes)
			} else {
				assert.Empty(t, got.Notes)
			}
		})
	}
}

// expectFind sets the client lookup of find, and checks the filter it uses.
func expectFind(t *testing.T, f *fixture, ref string, apps ...*managementv3.Client) {
	t.Helper()

	f.clients.EXPECT().
		List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientListPage, error) {
			if IsURL(ref) {
				require.NotNil(t, request.ExternalClientID)
				assert.Equal(t, ref, *request.ExternalClientID)
				assert.Nil(t, request.AgentID)
			} else {
				require.NotNil(t, request.AgentID)
				assert.Equal(t, ref, *request.AgentID)
				assert.Nil(t, request.ExternalClientID)
			}
			assert.Equal(t, clientFields, *request.Fields)

			return clientPage(apps...), nil
		})
}

func credentialOf(kid string) *managementv3.ClientCredential {
	return &managementv3.ClientCredential{Kid: ptr(kid)}
}

func jsonOf(value any) (string, error) {
	b, err := json.Marshal(value)

	return string(b), err
}

func TestListGranted(t *testing.T) {
	boom := errors.New("boom")

	muse := agentApp(museApp, museURL, museAgent)
	atlas := agentApp("client_atlas", atlasURL, "agt_atlas")

	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	atlasRecord := agentRecord("agt_atlas", "Atlas", atlasURL)

	shopGrant := grantOf("cgr_shop", shopAPI, "orders:read")
	otherGrant := grantOf("cgr_other", otherAPI, "refund")

	view := func(agentID, name, url, clientID string, apis Maybe[[]Grant]) GrantedAgent {
		return GrantedAgent{
			Agent: Agent{
				AgentID: agentID, Name: name, URL: url, RecordURL: url, ClientID: clientID,
				MetadataStatus: "available", DPoPRequired: known(false),
			},
			APIs: apis,
		}
	}

	shop := Grant{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}}
	other := Grant{ID: "cgr_other", Audience: otherAPI, Scopes: []string{"refund"}}

	tests := []struct {
		name      string
		narrowed  bool
		filterErr error
		records   []*managementv3.AgentResponseContent
		recordErr error
		apps      []*managementv3.Client
		appsErr   error
		grants    map[string][]*managementv3.ClientGrantResponseContent
		grantErr  map[string]error
		want      []GrantedAgent
		wantError string
	}{
		{
			name:    "every grant of every agent",
			records: []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			apps:    []*managementv3.Client{muse, atlas},
			grants: map[string][]*managementv3.ClientGrantResponseContent{
				museApp:        {shopGrant, otherGrant},
				"client_atlas": nil,
			},
			want: []GrantedAgent{
				view(museAgent, "Meta Muse", museURL, museApp, known([]Grant{shop, other})),
				view("agt_atlas", "Atlas", atlasURL, "client_atlas", known([]Grant{})),
			},
		},
		{
			name:     "narrowed, only the grants on the APIs the filter covers",
			narrowed: true,
			records:  []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			apps:     []*managementv3.Client{muse, atlas},
			grants: map[string][]*managementv3.ClientGrantResponseContent{
				museApp:        {shopGrant, otherGrant},
				"client_atlas": {otherGrant},
			},
			want: []GrantedAgent{
				view(museAgent, "Meta Muse", museURL, museApp, known([]Grant{shop})),
				view("agt_atlas", "Atlas", atlasURL, "client_atlas", known([]Grant{})),
			},
		},
		{
			name:    "a record without an app has no grants",
			records: []*managementv3.AgentResponseContent{museRecord},
			want: []GrantedAgent{{
				Agent: Agent{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"},
				APIs:  known([]Grant{}),
			}},
		},
		{
			name:    "the apps cannot be listed",
			records: []*managementv3.AgentResponseContent{museRecord},
			appsErr: boom,
			want: []GrantedAgent{{
				Agent: Agent{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"},
			}},
		},
		{
			name:     "the grants of one app cannot be read",
			records:  []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			apps:     []*managementv3.Client{muse, atlas},
			grants:   map[string][]*managementv3.ClientGrantResponseContent{museApp: {shopGrant}},
			grantErr: map[string]error{"client_atlas": boom},
			want: []GrantedAgent{
				view(museAgent, "Meta Muse", museURL, museApp, known([]Grant{shop})),
				view("agt_atlas", "Atlas", atlasURL, "client_atlas", Maybe[[]Grant]{}),
			},
		},
		{
			name:      "the records cannot be listed",
			recordErr: boom,
			wantError: "failed to list agents: boom",
		},
		{
			name:      "the audiences to narrow to cannot be read",
			narrowed:  true,
			filterErr: boom,
			records:   []*managementv3.AgentResponseContent{museRecord},
			wantError: "boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(test.records...), test.recordErr)

			// The resource servers are never read here: the filter is the caller's.
			var only AudienceFilter
			filterCalls := 0
			if test.narrowed {
				only = func(context.Context) (map[string]bool, error) {
					filterCalls++

					return map[string]bool{shopAPI: true}, test.filterErr
				}
			}

			if test.recordErr == nil && test.filterErr == nil {
				if test.appsErr != nil {
					f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.appsErr)
				} else {
					f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(test.apps...), nil)
				}

				for clientID, err := range test.grantErr {
					f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(clientID)}).Return(nil, err)
				}
				for clientID, grants := range test.grants {
					f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(clientID)}).Return(grantPage(grants...), nil)
				}
			}

			got, err := f.svc.ListGranted(t.Context(), only)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
			assert.Equal(t, map[bool]int{true: 1, false: 0}[test.narrowed], filterCalls, "the filter is called once")
		})
	}
}

func TestFindGranted(t *testing.T) {
	boom := errors.New("boom")
	record := agentRecord(museAgent, "Meta Muse", museURL)

	grants := func(f *fixture) {
		f.grants.EXPECT().
			List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp)}).
			Return(grantPage(grantOf("cgr_shop", shopAPI, "orders:read"), grantOf("cgr_other", otherAPI, "refund")), nil)
	}

	t.Run("the grants on every API", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museURL, agentApp(museApp, museURL, museAgent))
		f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(record, nil)
		grants(f)

		e, agent, err := f.svc.FindGranted(t.Context(), museURL, nil)

		require.NoError(t, err)
		assert.True(t, e.IsAgentApp())
		assert.Equal(t, museApp, agent.ClientID)
		assert.Len(t, agent.APIs.Value, 2)
		assert.True(t, agent.MayHaveGrants())
	})

	t.Run("narrowed", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museURL, agentApp(museApp, museURL, museAgent))
		f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(record, nil)
		grants(f)

		_, agent, err := f.svc.FindGranted(t.Context(), museURL, func(context.Context) (map[string]bool, error) {
			return map[string]bool{otherAPI: true}, nil
		})

		require.NoError(t, err)
		assert.Equal(t, []Grant{{ID: "cgr_other", Audience: otherAPI, Scopes: []string{"refund"}}}, agent.APIs.Value)
	})

	t.Run("no agent found", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museURL)

		e, _, err := f.svc.FindGranted(t.Context(), museURL, nil)

		assert.EqualError(t, err, "no agent found for "+museURL)
		assert.Nil(t, e)
	})

	t.Run("the audiences to narrow to cannot be read", func(t *testing.T) {
		f := newFixture(t)
		expectFind(t, f, museURL, agentApp(museApp, museURL, museAgent))
		f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(record, nil)

		_, _, err := f.svc.FindGranted(t.Context(), museURL, func(context.Context) (map[string]bool, error) { return nil, boom })

		assert.ErrorIs(t, err, boom)
	})
}

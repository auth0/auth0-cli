package poppy

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestList(t *testing.T) {
	boom := errors.New("boom")

	muse := agentApp(museApp, museURL, museAgent)
	atlas := agentApp("client_atlas", atlasURL, "agt_atlas")
	plain := &managementv3.Client{ClientID: ptr("client_plain"), Name: ptr("Plain")}

	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	atlasRecord := agentRecord("agt_atlas", "Atlas", atlasURL)

	shopGrant := grantOf("cgr_shop", shopAPI, "orders:read")
	otherGrant := grantOf("cgr_other", otherAPI, "refund")

	museAgentView := coreagents.GrantedAgent{
		Agent: coreagents.Agent{
			AgentID:        museAgent,
			Name:           "Meta Muse",
			URL:            museURL,
			RecordURL:      museURL,
			ClientID:       museApp,
			MetadataStatus: "available",
			DPoPRequired:   known(false),
		},
		APIs: known([]coreagents.Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}}}),
	}
	atlasAgentView := coreagents.GrantedAgent{
		Agent: coreagents.Agent{
			AgentID:        "agt_atlas",
			Name:           "Atlas",
			URL:            atlasURL,
			RecordURL:      atlasURL,
			ClientID:       "client_atlas",
			MetadataStatus: "available",
			DPoPRequired:   known(false),
		},
		APIs: known([]coreagents.Grant{}),
	}

	tests := []struct {
		name      string
		all       bool
		records   []*managementv3.AgentResponseContent
		recordErr error
		servers   []*managementv3.ResourceServer
		serverErr error
		apps      []*managementv3.Client
		appsErr   error
		grants    map[string][]*managementv3.ClientGrantResponseContent
		grantErr  map[string]error
		want      []coreagents.GrantedAgent
		wantError string
	}{
		{
			name:    "only the agents with a Poppy grant",
			records: []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			servers: []*managementv3.ResourceServer{shopServer(), otherServer()},
			apps:    []*managementv3.Client{muse, atlas, plain},
			grants: map[string][]*managementv3.ClientGrantResponseContent{
				museApp:        {shopGrant, otherGrant},
				"client_atlas": {otherGrant},
			},
			want: []coreagents.GrantedAgent{museAgentView},
		},
		{
			name:    "all agents",
			all:     true,
			records: []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			servers: []*managementv3.ResourceServer{shopServer()},
			apps:    []*managementv3.Client{muse, atlas},
			grants: map[string][]*managementv3.ClientGrantResponseContent{
				museApp:        {shopGrant},
				"client_atlas": nil,
			},
			want: []coreagents.GrantedAgent{museAgentView, atlasAgentView},
		},
		{
			name:    "an app without a record",
			records: nil,
			servers: []*managementv3.ResourceServer{shopServer()},
			apps:    []*managementv3.Client{muse},
			grants:  map[string][]*managementv3.ClientGrantResponseContent{museApp: {shopGrant}},
			want: []coreagents.GrantedAgent{{
				Agent: coreagents.Agent{
					Name:           "App " + museApp,
					URL:            museURL,
					ClientID:       museApp,
					MetadataStatus: "available",
					DPoPRequired:   known(false),
				},
				APIs: known([]coreagents.Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}}}),
			}},
		},
		{
			name:    "a record with another URL than its app",
			records: []*managementv3.AgentResponseContent{agentRecord(museAgent, "Meta Muse", atlasURL)},
			servers: []*managementv3.ResourceServer{shopServer()},
			apps:    []*managementv3.Client{muse},
			grants:  map[string][]*managementv3.ClientGrantResponseContent{museApp: {shopGrant}},
			want: []coreagents.GrantedAgent{func() coreagents.GrantedAgent {
				agent := museAgentView
				agent.RecordURL = atlasURL

				return agent
			}()},
		},
		{
			name:    "a record without an app is unlisted unless all is set",
			records: []*managementv3.AgentResponseContent{museRecord},
			servers: []*managementv3.ResourceServer{shopServer()},
			apps:    nil,
			want:    []coreagents.GrantedAgent{},
		},
		{
			name:    "a record without an app with all set",
			all:     true,
			records: []*managementv3.AgentResponseContent{museRecord},
			servers: []*managementv3.ResourceServer{shopServer()},
			apps:    nil,
			want:    []coreagents.GrantedAgent{{Agent: coreagents.Agent{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"}, APIs: known([]coreagents.Grant{})}},
		},
		{
			name:    "the apps cannot be listed",
			records: []*managementv3.AgentResponseContent{museRecord},
			servers: []*managementv3.ResourceServer{shopServer()},
			appsErr: boom,
			want:    []coreagents.GrantedAgent{{Agent: coreagents.Agent{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"}}},
		},
		{
			name:     "the grants of one app cannot be read",
			records:  []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			servers:  []*managementv3.ResourceServer{shopServer()},
			apps:     []*managementv3.Client{muse, atlas},
			grants:   map[string][]*managementv3.ClientGrantResponseContent{museApp: {shopGrant}},
			grantErr: map[string]error{"client_atlas": boom},
			want: []coreagents.GrantedAgent{
				museAgentView,
				{Agent: coreagents.Agent{AgentID: "agt_atlas", Name: "Atlas", URL: atlasURL, RecordURL: atlasURL, ClientID: "client_atlas", MetadataStatus: "available", DPoPRequired: known(false)}},
			},
		},
		{
			name:      "the records cannot be listed",
			recordErr: boom,
			wantError: "failed to list agents: boom",
		},
		{
			name:      "the APIs cannot be listed",
			servers:   nil,
			serverErr: boom,
			wantError: "failed to list APIs: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(test.records...), test.recordErr)

			if test.recordErr == nil {
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(test.servers...), test.serverErr)
			}

			if test.recordErr == nil && test.serverErr == nil {
				f.clients.EXPECT().
					List(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientListPage, error) {
						assert.Equal(t, clientFields, *request.Fields)
						assert.True(t, *request.IncludeFields)

						if test.appsErr != nil {
							return nil, test.appsErr
						}

						return clientPage(test.apps...), nil
					}).AnyTimes()

				for clientID, err := range test.grantErr {
					f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(clientID)}).Return(nil, err)
				}
				for clientID, grants := range test.grants {
					f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(clientID)}).Return(grantPage(grants...), nil)
				}
			}

			got, err := f.svc.List(t.Context(), test.all)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

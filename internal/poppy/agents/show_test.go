package agents

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// expectFind sets the client lookup of find, and checks the filter it uses.
func expectFind(t *testing.T, f *fixture, ref string, apps ...*managementv3.Client) {
	t.Helper()

	f.clients.EXPECT().
		List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientListPage, error) {
			if isURL(ref) {
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

func TestShow(t *testing.T) {
	boom := errors.New("boom")
	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	shopGrant := grantOf("cgr_shop", shopAPI, "orders:read")

	invalid := &managementv3.PreviewCimdMetadataResponseContent{
		Validation: &managementv3.CimdValidationResult{Violations: []string{"redirect_uris is empty"}},
	}

	tests := []struct {
		name         string
		ref          string
		app          *managementv3.Client
		recordErr    error
		grants       []*managementv3.ClientGrantResponseContent
		preview      *managementv3.PreviewCimdMetadataResponseContent
		previewErr   error
		credentials  []*managementv3.ClientCredential
		credErr      error
		wantValid    Maybe[bool]
		wantKeys     Maybe[bool]
		wantViolates []string
		wantWarnings []string
		wantNotes    []string
	}{
		{
			name:        "by URL with current keys",
			ref:         museURL,
			app:         agentApp(museApp, museURL, museAgent),
			grants:      []*managementv3.ClientGrantResponseContent{shopGrant},
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k2"), credentialOf("k1")},
			wantValid:   known(true),
			wantKeys:    known(true),
			wantWarnings: []string{
				"logo is small",
			},
		},
		{
			name:        "by agent ID with changed keys",
			ref:         museAgent,
			app:         agentApp(museApp, museURL, museAgent),
			grants:      []*managementv3.ClientGrantResponseContent{shopGrant},
			preview:     previewResponse(museApp),
			credentials: []*managementv3.ClientCredential{credentialOf("k1")},
			wantValid:   known(true),
			wantKeys:    known(false),
			wantWarnings: []string{
				"logo is small",
			},
		},
		{
			name:         "invalid document",
			ref:          museURL,
			app:          agentApp(museApp, museURL, museAgent),
			grants:       []*managementv3.ClientGrantResponseContent{shopGrant},
			preview:      invalid,
			wantValid:    known(false),
			wantViolates: []string{"redirect_uris is empty"},
		},
		{
			name:       "document check fails",
			ref:        museURL,
			app:        agentApp(museApp, museURL, museAgent),
			grants:     []*managementv3.ClientGrantResponseContent{shopGrant},
			previewErr: boom,
			wantNotes:  []string{"The metadata document could not be checked: failed to preview the agent metadata document: boom"},
		},
		{
			name:         "stored keys cannot be read",
			ref:          museURL,
			app:          agentApp(museApp, museURL, museAgent),
			grants:       []*managementv3.ClientGrantResponseContent{shopGrant},
			preview:      previewResponse(museApp),
			credErr:      boom,
			wantValid:    known(true),
			wantWarnings: []string{"logo is small"},
			wantNotes:    []string{"The keys Auth0 stored could not be read, so they were not compared: boom"},
		},
		{
			name:         "agent record gone",
			ref:          museURL,
			app:          agentApp(museApp, museURL, museAgent),
			recordErr:    apiError(404, `{"errorCode":"inexistent_agent"}`),
			grants:       []*managementv3.ClientGrantResponseContent{shopGrant},
			preview:      previewResponse(museApp),
			credentials:  []*managementv3.ClientCredential{credentialOf("k1"), credentialOf("k2")},
			wantValid:    known(true),
			wantKeys:     known(true),
			wantWarnings: []string{"logo is small"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			expectFind(t, f, test.ref, test.app)
			f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(museRecord, test.recordErr)
			f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(shopServer()), nil)
			f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp)}).Return(grantPage(test.grants...), nil)
			f.clients.EXPECT().
				PreviewCimdMetadata(gomock.Any(), &managementv3.PreviewCimdMetadataRequestContent{ExternalClientID: museURL}).
				Return(test.preview, test.previewErr)

			if test.wantKeys.Known || test.credErr != nil {
				f.credentials.EXPECT().List(gomock.Any(), museApp).Return(test.credentials, test.credErr)
			}

			got, err := f.svc.Show(t.Context(), test.ref)

			require.NoError(t, err)
			assert.Equal(t, museApp, got.ClientID)
			assert.Equal(t, museURL, got.URL)
			assert.Equal(t, test.wantValid, got.MetadataCheck.Valid)
			assert.Equal(t, test.wantKeys, got.KeysCurrent)
			assert.ElementsMatch(t, test.wantViolates, got.MetadataCheck.Violations)
			assert.ElementsMatch(t, test.wantWarnings, got.MetadataCheck.Warnings)
			assert.ElementsMatch(t, test.wantNotes, got.Notes)
		})
	}
}

func TestShowRejects(t *testing.T) {
	otherGrant := grantOf("cgr_other", otherAPI, "refund")
	plain := &managementv3.Client{ClientID: ptr("client_plain"), Name: ptr("Plain"), AgentID: ptr("agt_plain")}

	tests := []struct {
		name      string
		ref       string
		app       *managementv3.Client
		grants    []*managementv3.ClientGrantResponseContent
		wantError string
	}{
		{name: "no agent app", ref: museURL, wantError: "no agent found for " + museURL},
		{name: "not an agent app", ref: "agt_plain", app: plain, wantError: "agt_plain isn't a Poppy agent"},
		{
			name:      "no Poppy grant",
			ref:       museURL,
			app:       agentApp(museApp, museURL, museAgent),
			grants:    []*managementv3.ClientGrantResponseContent{otherGrant},
			wantError: museURL + " isn't a Poppy agent",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			expectFind(t, f, test.ref, appsOf(test.app)...)

			if test.app != nil {
				f.agents.EXPECT().Read(gomock.Any(), test.app.GetAgentID()).Return(agentRecord(test.app.GetAgentID(), "Record", museURL), nil)
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(shopServer()), nil)
				f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: test.app.ClientID}).Return(grantPage(test.grants...), nil)
			}

			got, err := f.svc.Show(t.Context(), test.ref)

			assert.EqualError(t, err, test.wantError)
			assert.Nil(t, got)
		})
	}
}

func appsOf(app *managementv3.Client) []*managementv3.Client {
	if app == nil {
		return nil
	}

	return []*managementv3.Client{app}
}

func TestShowErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("the client lookup fails", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, boom)

		_, err := f.svc.Show(t.Context(), museURL)

		assert.EqualError(t, err, "failed to look up "+museURL+": boom")
	})

	t.Run("the record read fails", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(agentApp(museApp, museURL, museAgent)), nil)
		f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(nil, apiError(500, "server error"))

		_, err := f.svc.Show(t.Context(), museURL)

		assert.ErrorContains(t, err, "failed to read agent "+museAgent)
	})

	t.Run("the APIs cannot be listed", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(agentApp(museApp, museURL, museAgent)), nil)
		f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(agentRecord(museAgent, "Meta Muse", museURL), nil)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, boom)

		_, err := f.svc.Show(t.Context(), museURL)

		assert.EqualError(t, err, "failed to list APIs: boom")
	})
}

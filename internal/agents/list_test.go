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

func TestLinkAndDescribe(t *testing.T) {
	boom := errors.New("boom")

	muse := agentApp(museApp, museURL, museAgent)
	atlas := agentApp("client_atlas", atlasURL, "agt_atlas")
	plain := &managementv3.Client{ClientID: ptr("client_plain"), Name: ptr("Plain")}

	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	atlasRecord := agentRecord("agt_atlas", "Atlas", atlasURL)

	museView := Agent{
		AgentID:        museAgent,
		Name:           "Meta Muse",
		URL:            museURL,
		RecordURL:      museURL,
		ClientID:       museApp,
		MetadataStatus: "available",
		DPoPRequired:   Known(false),
	}
	atlasView := Agent{
		AgentID:        "agt_atlas",
		Name:           "Atlas",
		URL:            atlasURL,
		RecordURL:      atlasURL,
		ClientID:       "client_atlas",
		MetadataStatus: "available",
		DPoPRequired:   Known(false),
	}

	tests := []struct {
		name      string
		records   []*managementv3.AgentResponseContent
		recordErr error
		apps      []*managementv3.Client
		appsErr   error
		want      []Agent
		wantError string
	}{
		{
			name:    "every agent, whatever its access",
			records: []*managementv3.AgentResponseContent{museRecord, atlasRecord},
			apps:    []*managementv3.Client{muse, atlas, plain},
			want:    []Agent{museView, atlasView},
		},
		{
			name:    "an app without a record",
			records: nil,
			apps:    []*managementv3.Client{muse},
			want: []Agent{{
				Name:           "App " + museApp,
				URL:            museURL,
				ClientID:       museApp,
				MetadataStatus: "available",
				DPoPRequired:   Known(false),
			}},
		},
		{
			name:    "a record without an app",
			records: []*managementv3.AgentResponseContent{museRecord},
			want:    []Agent{{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"}},
		},
		{
			name:    "the apps cannot be listed",
			records: []*managementv3.AgentResponseContent{museRecord},
			appsErr: boom,
			want:    []Agent{{AgentID: museAgent, Name: "Meta Muse", URL: museURL, RecordURL: museURL, MetadataStatus: "unknown"}},
		},
		{
			name:      "the records cannot be listed",
			recordErr: boom,
			wantError: "failed to list agents: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(test.records...), test.recordErr)

			if test.recordErr == nil {
				f.clients.EXPECT().
					List(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientListPage, error) {
						assert.Equal(t, clientFields, *request.Fields)
						assert.True(t, *request.IncludeFields)

						if test.appsErr != nil {
							return nil, test.appsErr
						}

						return clientPage(test.apps...), nil
					})
			}

			records, err := f.svc.Records(t.Context())

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			require.NoError(t, err)

			entries, _ := f.svc.Link(t.Context(), records)

			got := []Agent{}
			for _, e := range entries {
				got = append(got, Describe(e))
			}

			assert.Equal(t, test.want, got)
		})
	}
}

func TestFindUsesOnlyTheClientAskedFor(t *testing.T) {
	muse := agentApp(museApp, museURL, museAgent)
	atlas := agentApp("client_atlas", atlasURL, "agt_atlas")

	tests := []struct {
		name string
		ref  string
		// The clients a server answers with, whatever the filter says.
		listed   []*managementv3.Client
		wantApp  string
		wantNone bool
	}{
		{name: "by URL", ref: museURL, listed: []*managementv3.Client{atlas, muse}, wantApp: museApp},
		{name: "by agent ID", ref: museAgent, listed: []*managementv3.Client{atlas, muse}, wantApp: museApp},
		{name: "no URL matches", ref: museURL, listed: []*managementv3.Client{atlas}, wantNone: true},
		{name: "no agent ID matches", ref: museAgent, listed: []*managementv3.Client{atlas}, wantNone: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(test.listed...), nil)

			if !test.wantNone {
				f.agents.EXPECT().Read(gomock.Any(), museAgent).Return(agentRecord(museAgent, "Meta Muse", museURL), nil)
			}

			got, err := f.svc.Find(t.Context(), test.ref)

			require.NoError(t, err)
			if test.wantNone {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, test.wantApp, got.Client.GetClientID())
		})
	}
}

func TestCheckMetadata(t *testing.T) {
	boom := errors.New("boom")
	invalid := &managementv3.PreviewCimdMetadataResponseContent{
		Validation: &managementv3.CimdValidationResult{Violations: []string{"redirect_uris is empty"}},
	}

	credentials := func(kids ...string) []*managementv3.ClientCredential {
		out := make([]*managementv3.ClientCredential, len(kids))
		for i, kid := range kids {
			out[i] = &managementv3.ClientCredential{Kid: ptr(kid)}
		}

		return out
	}

	// The keys of a real agent whose document sets token_endpoint_auth_signing_alg
	// to ES256, so that Auth0 stores the EC key and leaves the RSA key out.
	const (
		ecKID  = "rk0mqGQZugkKsfBGuqE07e1rzvDclJoHyMLUkpGlP9Q"
		rsaKID = "6igZefSYqvIV_3q_3DW55HOZo-I4eX6zzJ_YSQEsdh4"
	)

	withKeys := func(keys ...Key) *managementv3.PreviewCimdMetadataResponseContent {
		resp := previewResponse("")
		credentials := make([]*managementv3.CimdMappedPrivateKeyJwtCredential, len(keys))
		for i, key := range keys {
			credentials[i] = &managementv3.CimdMappedPrivateKeyJwtCredential{CredentialType: "public_key", Kid: key.KID, Alg: key.Alg}
		}
		resp.MappedFields.ClientAuthenticationMethods.PrivateKeyJwt.Credentials = credentials

		return resp
	}

	ecKey, rsaKey := Key{KID: ecKID, Alg: "ES256"}, Key{KID: rsaKID, Alg: "RS256"}

	tests := []struct {
		name        string
		preview     *managementv3.PreviewCimdMetadataResponseContent
		previewErr  error
		stored      []*managementv3.ClientCredential
		storedErr   error
		wantValid   Maybe[bool]
		wantKeys    Maybe[bool]
		wantViolate []string
		wantNote    string
		wantUsable  int
	}{
		{
			name:       "the stored keys are current",
			preview:    previewResponse(museApp),
			stored:     credentials("k2", "k1"),
			wantValid:  Known(true),
			wantKeys:   Known(true),
			wantUsable: 2,
		},
		{
			name:       "the document signs with ES256, and only the EC key is listed and stored",
			preview:    withKeys(ecKey),
			stored:     credentials(ecKID),
			wantValid:  Known(true),
			wantKeys:   Known(true),
			wantUsable: 1,
		},
		{
			name:       "the RSA key was rotated",
			preview:    withKeys(rsaKey),
			stored:     credentials("rotated"),
			wantValid:  Known(true),
			wantKeys:   Known(false),
			wantUsable: 1,
		},
		{
			name:      "the document has no key that Auth0 stores",
			preview:   withKeys(),
			wantValid: Known(true),
			wantKeys:  Known(true),
		},
		{
			name:       "the stored keys changed",
			preview:    previewResponse(museApp),
			stored:     credentials("k1"),
			wantValid:  Known(true),
			wantKeys:   Known(false),
			wantUsable: 2,
		},
		{
			name:        "an invalid document",
			preview:     invalid,
			wantValid:   Known(false),
			wantViolate: []string{"redirect_uris is empty"},
		},
		{
			name:       "the document cannot be checked",
			previewErr: boom,
			wantNote:   "The metadata document could not be checked: failed to preview the agent metadata document: boom",
		},
		{
			name:      "the stored keys cannot be read",
			preview:   previewResponse(museApp),
			storedErr: boom,
			wantValid: Known(true),
			wantNote:  "The keys Auth0 stored could not be read, so they were not compared: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.clients.EXPECT().PreviewCimdMetadata(gomock.Any(), gomock.Any()).Return(test.preview, test.previewErr)

			if test.preview != nil && test.preview.Validation.GetValid() {
				f.credentials.EXPECT().List(gomock.Any(), museApp).Return(test.stored, test.storedErr)
			}

			detail := NewDetail(Describe(Entry{Client: agentApp(museApp, museURL, museAgent)}))
			f.svc.CheckMetadata(t.Context(), detail)

			assert.Equal(t, test.wantValid, detail.MetadataCheck.Valid)
			assert.Equal(t, test.wantKeys, detail.KeysCurrent)

			if test.wantViolate != nil {
				assert.Equal(t, test.wantViolate, detail.MetadataCheck.Violations)
			}

			if test.wantNote != "" {
				assert.Equal(t, []string{test.wantNote}, detail.Notes)
			} else {
				assert.Empty(t, detail.Notes)
			}

			if test.wantKeys.Known {
				assert.Equal(t, test.wantUsable, detail.MetadataCheck.UsableKeys)
			}
		})
	}
}

func TestRecordByURL(t *testing.T) {
	boom := errors.New("boom")
	muse := agentRecord(museAgent, "Meta Muse", museURL)
	twin := agentRecord("agt_twin", "Twin", museURL)
	atlas := agentRecord("agt_atlas", "Atlas", atlasURL)

	tests := []struct {
		name      string
		records   []*managementv3.AgentResponseContent
		recordErr error
		want      *managementv3.AgentResponseContent
		wantError string
	}{
		{name: "the record with the URL", records: []*managementv3.AgentResponseContent{atlas, muse}, want: muse},
		{name: "the first of several", records: []*managementv3.AgentResponseContent{atlas, muse, twin}, want: muse},
		{name: "no record has the URL", records: []*managementv3.AgentResponseContent{atlas}},
		{name: "the records cannot be listed", recordErr: boom, wantError: "failed to list agents: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(test.records...), test.recordErr)

			got, err := f.svc.RecordByURL(t.Context(), museURL)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			require.NoError(t, err)
			assert.Same(t, test.want, got)
		})
	}
}

func TestLinkApps(t *testing.T) {
	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	atlasRecord := agentRecord("agt_atlas", "Atlas", atlasURL)
	m2m := &managementv3.Client{ClientID: ptr("client_m2m"), AgentID: ptr("agt_atlas")}
	muse := agentApp(museApp, museURL, museAgent)
	plain := &managementv3.Client{ClientID: ptr("client_plain")}

	t.Run("a client of any type is the app of its record, in the order of the records", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(plain, m2m, muse), nil).Times(1)

		entries, err := f.svc.LinkApps(t.Context(), []*managementv3.AgentResponseContent{atlasRecord, museRecord})

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Same(t, m2m, entries[0].Client)
		assert.Same(t, muse, entries[1].Client)
	})

	t.Run("a record without a client has none, and the clients cannot be listed", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(plain), nil)

		entries, err := f.svc.LinkApps(t.Context(), []*managementv3.AgentResponseContent{museRecord})

		assert.NoError(t, err)
		assert.Nil(t, entries[0].Client)

		f = newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		entries, err = f.svc.LinkApps(t.Context(), []*managementv3.AgentResponseContent{museRecord})

		assert.Error(t, err)
		require.Len(t, entries, 1)
		assert.Nil(t, entries[0].Client)
	})
}

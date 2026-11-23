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
		name            string
		violations      []string
		wantNote        bool
		wantUnreachable bool
	}{
		{name: "key violation", violations: []string{"jwks_uri must serve at least one key"}, wantNote: true},
		{name: "algorithm violation", violations: []string{"unsupported alg ES256"}, wantNote: true},
		{name: "signing algorithm violation", violations: []string{"token_endpoint_auth_signing_alg must be one of the supported algorithms"}, wantNote: true},
		{name: "other violation", violations: []string{"redirect_uris is empty"}},
		{name: "retrieval error", violations: []string{"Unexpected error during retrieval for " + museURL}, wantUnreachable: true},
		{name: "retrieval status", violations: []string{"Retrieval failed for " + museURL + " with status 404"}, wantUnreachable: true},
		{name: "retrieval failed twice", violations: []string{"Unexpected error during retrieval for " + museURL, "Retrieval failed for " + museURL + " with status 404"}, wantUnreachable: true},
		{name: "retrieval and another violation", violations: []string{"Retrieval failed for " + museURL + " with status 404", "redirect_uris is empty"}},
		{name: "no violation"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := &InvalidDocumentError{URL: museURL, Violations: test.violations}

			assert.Equal(t, "the agent metadata document at "+museURL+" is not valid", err.Error())
			assert.Equal(t, test.wantNote, strings.Contains(err.Advice(), keysNote))
			assert.Equal(t, test.wantUnreachable, err.Unreachable())

			for _, violation := range test.violations {
				assert.Contains(t, err.Advice(), violation)
			}
		})
	}
}

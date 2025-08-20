package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestAPIsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		apis         []*management.ResourceServer
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("some-id-1"),
					Identifier: auth0.String("some-audience-1"),
					Name:       auth0.String("some-name-1"),
				},
				{
					ID:         auth0.String("some-id-2"),
					Identifier: auth0.String("some-audience-2"),
					Name:       auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-audience-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-audience-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "APIs with subject type authorization",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("api-id-1"),
					Identifier: auth0.String("https://api.example.com"),
					Name:       auth0.String("Example API"),
					SubjectTypeAuthorization: &management.ResourceServerSubjectTypeAuthorization{
						User: &management.ResourceServerSubjectTypeAuthorizationUser{
							Policy: auth0.String("allow_all"),
						},
						Client: &management.ResourceServerSubjectTypeAuthorizationClient{
							Policy: auth0.String("deny_all"),
						},
					},
				},
				{
					ID:         auth0.String("api-id-2"),
					Identifier: auth0.String("https://secure-api.example.com"),
					Name:       auth0.String("Secure API"),
					SubjectTypeAuthorization: &management.ResourceServerSubjectTypeAuthorization{
						User: &management.ResourceServerSubjectTypeAuthorizationUser{
							Policy: auth0.String("require_client_grant"),
						},
						Client: &management.ResourceServerSubjectTypeAuthorizationClient{
							Policy: auth0.String("deny_all"),
						},
					},
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "Example API (https://api.example.com)", options[0].label)
				assert.Equal(t, "api-id-1", options[0].value)
				assert.Equal(t, "Secure API (https://secure-api.example.com)", options[1].label)
				assert.Equal(t, "api-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "no apis",
			apis: []*management.ResourceServer{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no APIs to choose from. Create one by running: `auth0 apis create`")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			apiAPI := mock.NewMockResourceServerAPI(ctrl)
			apiAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ResourceServerList{
					ResourceServers: test.apis}, test.apiError)

			cli := &cli{
				api: &auth0.API{ResourceServer: apiAPI},
			}

			options, err := cli.apiPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestParseSubjectTypeAuthorization(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult *management.ResourceServerSubjectTypeAuthorization
		expectedError  string
	}{
		{
			name:  "valid complete JSON",
			input: `{"user":{"policy":"allow_all"},"client":{"policy":"deny_all"}}`,
			expectedResult: &management.ResourceServerSubjectTypeAuthorization{
				User: &management.ResourceServerSubjectTypeAuthorizationUser{
					Policy: auth0.String("allow_all"),
				},
				Client: &management.ResourceServerSubjectTypeAuthorizationClient{
					Policy: auth0.String("deny_all"),
				},
			},
		},
		{
			name:  "valid user only JSON",
			input: `{"user":{"policy":"require_client_grant"}}`,
			expectedResult: &management.ResourceServerSubjectTypeAuthorization{
				User: &management.ResourceServerSubjectTypeAuthorizationUser{
					Policy: auth0.String("require_client_grant"),
				},
			},
		},
		{
			name:  "valid client only JSON",
			input: `{"client":{"policy":"deny_all"}}`,
			expectedResult: &management.ResourceServerSubjectTypeAuthorization{
				Client: &management.ResourceServerSubjectTypeAuthorizationClient{
					Policy: auth0.String("deny_all"),
				},
			},
		},
		{
			name:           "empty string input",
			input:          "",
			expectedResult: nil,
		},
		{
			name:          "invalid JSON syntax",
			input:         `{"user":{"policy":"allow_all"`,
			expectedError: "invalid JSON for subject type authorization",
		},
		{
			name:          "invalid user policy",
			input:         `{"user":{"policy":"invalid_policy"}}`,
			expectedError: "invalid user policy: invalid_policy. Valid options: allow_all, deny_all, require_client_grant",
		},
		{
			name:          "invalid client policy",
			input:         `{"client":{"policy":"allow_all"}}`,
			expectedError: "invalid client policy: allow_all. Valid options: deny_all, require_client_grant",
		},
		{
			name:  "valid JSON with unsupported fields",
			input: `{"user":{"policy":"allow_all","extra":"field"},"client":{"policy":"deny_all"}}`,
			expectedResult: &management.ResourceServerSubjectTypeAuthorization{
				User: &management.ResourceServerSubjectTypeAuthorizationUser{
					Policy: auth0.String("allow_all"),
				},
				Client: &management.ResourceServerSubjectTypeAuthorizationClient{
					Policy: auth0.String("deny_all"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseSubjectTypeAuthorization(test.input)

			if test.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}

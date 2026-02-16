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
	"github.com/auth0/auth0-cli/internal/config"
)

func TestStringPtr(t *testing.T) {
	t.Run("returns nil when input is nil", func(t *testing.T) {
		type CustomString string
		var nilPtr *CustomString = nil
		result := stringPtr(nilPtr)
		assert.Nil(t, result)
	})

	t.Run("converts custom string pointer to string pointer", func(t *testing.T) {
		type CustomString string
		value := CustomString("test-value")
		result := stringPtr(&value)
		assert.NotNil(t, result)
		assert.Equal(t, "test-value", *result)
	})

	t.Run("converts regular string pointer to string pointer", func(t *testing.T) {
		value := "regular-string"
		result := stringPtr(&value)
		assert.NotNil(t, result)
		assert.Equal(t, "regular-string", *result)
	})
}

func TestBuildOauthTokenURL(t *testing.T) {
	url := BuildOauthTokenURL("cli-demo.us.auth0.com")
	assert.Equal(t, "https://cli-demo.us.auth0.com/oauth/token", url)
}

func TestBuildOauthTokenParams(t *testing.T) {
	params := BuildOauthTokenParams("some-client-id", "some-client-secret", "https://cli-demo.auth0.us.auth0.com/api/v2/")
	assert.Equal(t, "audience=https%3A%2F%2Fcli-demo.auth0.us.auth0.com%2Fapi%2Fv2%2F&client_id=some-client-id&client_secret=some-client-secret&grant_type=client_credentials", params.Encode())
}

func TestHasLocalCallbackURL(t *testing.T) {
	assert.False(t, hasLocalCallbackURL(&management.Client{
		Callbacks: &[]string{"http://localhost:3000"},
	}))
	assert.True(t, hasLocalCallbackURL(&management.Client{
		Callbacks: &[]string{"http://localhost:8484"},
	}))
}

func TestFormatManageTenantURL(t *testing.T) {
	assert.Empty(t, formatManageTenantURL("", &config.Config{}))

	assert.Empty(t, formatManageTenantURL("invalid-tenant-url", &config.Config{}))

	assert.Empty(t, formatManageTenantURL("valid-tenant-url-not-in-config.us.auth0", &config.Config{}))

	tenantDomain := "some-tenant.us.auth0"
	assert.Equal(t, formatManageTenantURL(tenantDomain, &config.Config{Tenants: map[string]config.Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/us/some-tenant/")

	tenantDomain = "some-eu-tenant.eu.auth0.com"
	assert.Equal(t, formatManageTenantURL(tenantDomain, &config.Config{Tenants: map[string]config.Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/eu/some-tenant/")

	tenantDomain = "dev-tti06f6y.auth0.com"
	assert.Equal(t, formatManageTenantURL(tenantDomain, &config.Config{Tenants: map[string]config.Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/us/some-tenant/")
}

func TestContainsStr(t *testing.T) {
	assert.False(t, containsStr([]string{"string-1", "string-2"}, "string-3"))
	assert.True(t, containsStr([]string{"string-1", "string-2"}, "string-1"))
}

func TestGenerateState(t *testing.T) {
	state, err := generateState(0)
	assert.Equal(t, "", state)
	assert.Nil(t, err)

	state, err = generateState(cliLoginTestingStateSize)
	assert.IsType(t, "string", state)
	assert.Nil(t, err)
}

func TestAddLocalCallbackURLToClient(t *testing.T) {
	tests := []struct {
		name         string
		intialClient *management.Client
		finalClient  *management.Client
		apiError     error
		assertOutput func(t testing.TB, result bool)
		assertError  func(t testing.TB, err error)
	}{
		{
			name:         "adds the callback",
			intialClient: &management.Client{ClientID: auth0.String("")},
			finalClient: &management.Client{
				Callbacks: &[]string{cliLoginTestingCallbackURL},
			},
			assertOutput: func(t testing.TB, result bool) {
				assert.True(t, result)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "does not add the callback when alredy present",
			intialClient: &management.Client{
				ClientID: auth0.String(""),
				Callbacks: &[]string{
					"http://localhost:3000",
					cliLoginTestingCallbackURL,
				},
			},
			assertOutput: func(t testing.TB, result bool) {
				assert.False(t, result)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:         "returns the API error",
			intialClient: &management.Client{ClientID: auth0.String("")},
			finalClient: &management.Client{
				Callbacks: &[]string{cliLoginTestingCallbackURL},
			},
			apiError: errors.New("error"),
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			timesAPIShouldBeCalled := 1
			if test.finalClient == nil {
				timesAPIShouldBeCalled = 0
			}

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Update(context.Background(), gomock.Any(), gomock.Eq(test.finalClient)).
				Return(test.apiError).
				Times(timesAPIShouldBeCalled)

			result, err := addLocalCallbackURLToClient(context.Background(), clientAPI, test.intialClient)

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, result)
			}
		})
	}
}

func TestRemoveLocalCallbackURLToClient(t *testing.T) {
	tests := []struct {
		name         string
		intialClient *management.Client
		finalClient  *management.Client
		apiError     error
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "removes the callback",
			intialClient: &management.Client{
				ClientID: auth0.String(""),
				Callbacks: &[]string{
					"http://localhost:3000",
					cliLoginTestingCallbackURL,
				},
			},
			finalClient: &management.Client{
				Callbacks: &[]string{"http://localhost:3000"},
			},
			assertError: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "does not remove the callback when not present",
			intialClient: &management.Client{
				ClientID:  auth0.String(""),
				Callbacks: &[]string{"http://localhost:3000"},
			},
			assertError: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "does not remove the callback when there are no other callbacks",
			intialClient: &management.Client{
				ClientID:  auth0.String(""),
				Callbacks: &[]string{cliLoginTestingCallbackURL},
			},
			assertError: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "returns the API error",
			intialClient: &management.Client{
				ClientID: auth0.String(""),
				Callbacks: &[]string{
					"http://localhost:3000",
					cliLoginTestingCallbackURL,
				},
			},
			finalClient: &management.Client{
				Callbacks: &[]string{"http://localhost:3000"},
			},
			apiError: errors.New("error"),
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			timesAPIShouldBeCalled := 1
			if test.finalClient == nil {
				timesAPIShouldBeCalled = 0
			}

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Update(context.Background(), gomock.Any(), gomock.Eq(test.finalClient)).
				Return(test.apiError).
				Times(timesAPIShouldBeCalled)

			err := removeLocalCallbackURLFromClient(context.Background(), clientAPI, test.intialClient)

			test.assertError(t, err)
		})
	}
}

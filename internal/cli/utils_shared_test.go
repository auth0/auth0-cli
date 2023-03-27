package cli

import (
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

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
	assert.Empty(t, formatManageTenantURL("", config{}))

	assert.Empty(t, formatManageTenantURL("invalid-tenant-url", config{}))

	assert.Empty(t, formatManageTenantURL("valid-tenant-url-not-in-config.us.auth0", config{}))

	tenantDomain := "some-tenant.us.auth0"
	assert.Equal(t, formatManageTenantURL(tenantDomain, config{Tenants: map[string]Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/us/some-tenant/")

	tenantDomain = "some-eu-tenant.eu.auth0.com"
	assert.Equal(t, formatManageTenantURL(tenantDomain, config{Tenants: map[string]Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/eu/some-tenant/")

	tenantDomain = "dev-tti06f6y.auth0.com"
	assert.Equal(t, formatManageTenantURL(tenantDomain, config{Tenants: map[string]Tenant{tenantDomain: {Name: "some-tenant"}}}), "https://manage.auth0.com/dashboard/us/some-tenant/")
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
	assert.Equal(t, cliLoginTestingStateSize, len(state))
	assert.Nil(t, err)
}

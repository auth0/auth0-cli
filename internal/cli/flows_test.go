package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/config"
)

func TestApplyRawNameOverride(t *testing.T) {
	body := json.RawMessage(`{"name":"Original","actions":[{"id":"a1","type":"HTTP"}]}`)

	got, err := applyRawNameOverride(body, "Renamed")
	require.NoError(t, err)

	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got, &obj))
	assert.JSONEq(t, `"Renamed"`, string(obj["name"]))
	assert.Contains(t, string(obj["actions"]), `"HTTP"`)
}

func TestApplyRawNameOverrideNoopWhenEmpty(t *testing.T) {
	body := json.RawMessage(`{"actions":[]}`)

	got, err := applyRawNameOverride(body, "")
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestApplyRawNameOverrideRejectsNonObject(t *testing.T) {
	_, err := applyRawNameOverride(json.RawMessage(`[]`), "New")
	assert.ErrorContains(t, err, "cannot unmarshal array")
}

func TestFormatBuilderPageURL(t *testing.T) {
	cfg := &config.Config{
		Tenants: config.Tenants{
			"example.us.auth0.com":   {Name: "example"},
			"my-tenant.eu.auth0.com": {Name: "my-tenant"},
			"dev-tti06f6y.auth0.com": {Name: "dev-tti06f6y"},
			"no-name.us.auth0.com":   {Name: ""},
		},
	}

	tests := []struct {
		name     string
		tenant   string
		path     string
		expected string
	}{
		{
			name:     "builds a flow edit URL",
			tenant:   "example.us.auth0.com",
			path:     "flows/af_123/edit",
			expected: "https://forms.auth0.com/tenants/us/example/flows/af_123/edit",
		},
		{
			name:     "builds a vault app URL in a non-us region",
			tenant:   "my-tenant.eu.auth0.com",
			path:     "vault/apps/HTTP/edit",
			expected: "https://forms.auth0.com/tenants/eu/my-tenant/vault/apps/HTTP/edit",
		},
		{
			name:     "defaults to us for a three-part PUS1 domain",
			tenant:   "dev-tti06f6y.auth0.com",
			path:     "vault/apps/JWT/edit",
			expected: "https://forms.auth0.com/tenants/us/dev-tti06f6y/vault/apps/JWT/edit",
		},
		{
			name:     "returns empty when the path is missing",
			tenant:   "example.us.auth0.com",
			path:     "",
			expected: "",
		},
		{
			name:     "returns empty when the tenant is missing",
			tenant:   "",
			path:     "flows/af_123/edit",
			expected: "",
		},
		{
			name:     "returns empty when the domain has too few parts",
			tenant:   "invalid",
			path:     "flows/af_123/edit",
			expected: "",
		},
		{
			name:     "returns empty when the tenant name is unknown",
			tenant:   "no-name.us.auth0.com",
			path:     "flows/af_123/edit",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, formatBuilderPageURL(test.tenant, cfg, test.path))
		})
	}
}

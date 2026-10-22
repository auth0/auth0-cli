package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

func TestResolveRegistries(t *testing.T) {
	builtin := registry.Registry{Name: "auth0", URL: registry.DefaultRegistryURL}

	tests := []struct {
		name       string
		configured []config.RegistryConfig
		want       []registry.Registry
		wantErr    string
	}{
		{name: "nothing configured", want: []registry.Registry{builtin}},
		{
			name:       "a mirror of the auth0 registry",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "https://mirror.example.com/index.json"}},
			want:       []registry.Registry{{Name: "auth0", URL: "https://mirror.example.com/index.json"}},
		},
		{
			name:       "a mirror without a URL",
			configured: []config.RegistryConfig{{Name: "auth0"}},
			wantErr:    "registries[0].url:",
		},
		{
			name:       "a mirror over plain http",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "http://mirror.example.com/index.json"}},
			wantErr:    "registries[0].url: the URL must use https",
		},
		{
			name:       "a mirror that is not a URL",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "mirror.example.com/index.json"}},
			wantErr:    "registries[0].url: the URL must use https",
		},
		{
			name:       "a mirror with a password",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "https://user:secret@mirror.example.com/index.json"}},
			wantErr:    "registries[0].url: the URL must use https",
		},
		{
			name:       "the auth0 registry twice",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "https://a.example.com/index.json"}, {Name: "auth0", URL: "https://b.example.com/index.json"}},
			wantErr:    "registries[1]: registry \"auth0\" is configured more than once",
		},
		{
			name:       "a custom registry",
			configured: []config.RegistryConfig{{Name: "acme", URL: "https://acme.example.com/index.json"}},
			wantErr:    "custom registries are not supported yet",
		},
		{
			name:       "a custom registry next to the auth0 registry",
			configured: []config.RegistryConfig{{Name: "auth0", URL: "https://a.example.com/index.json"}, {Name: "acme", URL: "https://acme.example.com/index.json"}},
			wantErr:    "registries[1]: cannot use registry \"acme\", custom registries are not supported yet",
		},
		{
			name:       "a registry without a name",
			configured: []config.RegistryConfig{{URL: "https://acme.example.com/index.json"}},
			wantErr:    "custom registries are not supported yet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := registry.ResolveRegistries(test.configured)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

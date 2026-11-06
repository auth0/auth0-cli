package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

// The in-memory tenant of the flow tests and the adapter over the SDK are two implementations of the
// same stores, so what a flow test sees must be what the API gets. ClientSettings says that on an
// update the settings left empty or nil are not sent. Each case runs against both. The fake must keep
// the current value, and the adapter must leave the field out of the request.
func TestClientUpdateContract_ASettingLeftEmptyIsNotSent(t *testing.T) {
	t.Parallel()

	current := portals.ClientSettings{
		Name:              "Acme",
		Callbacks:         []string{"https://acme/cb"},
		AllowedLogoutURLs: []string{"https://acme/out"},
		GrantTypes:        []string{"authorization_code", "refresh_token"},
	}

	tests := []struct {
		name   string
		update portals.ClientSettings
		// Field is the field of the request. Read returns the same setting from an application.
		field string
		read  func(portals.ClientSettings) any
		// Want is what the application has after the update. Sent says if the request carries it.
		want any
		sent bool
	}{
		{
			name:   "grant types: an empty list",
			update: portals.ClientSettings{GrantTypes: []string{}},
			field:  "grant_types",
			read:   func(settings portals.ClientSettings) any { return settings.GrantTypes },
			want:   current.GrantTypes,
		},
		{
			name:   "grant types: nil",
			update: portals.ClientSettings{Name: "Renamed"},
			field:  "grant_types",
			read:   func(settings portals.ClientSettings) any { return settings.GrantTypes },
			want:   current.GrantTypes,
		},
		{
			name:   "callbacks: an empty list",
			update: portals.ClientSettings{Callbacks: []string{}},
			field:  "callbacks",
			read:   func(settings portals.ClientSettings) any { return settings.Callbacks },
			want:   current.Callbacks,
		},
		{
			name:   "logout URLs: an empty list",
			update: portals.ClientSettings{AllowedLogoutURLs: []string{}},
			field:  "allowed_logout_urls",
			read:   func(settings portals.ClientSettings) any { return settings.AllowedLogoutURLs },
			want:   current.AllowedLogoutURLs,
		},
		{
			name:   "name: an empty string",
			update: portals.ClientSettings{Callbacks: []string{"https://acme/other"}},
			field:  "name",
			read:   func(settings portals.ClientSettings) any { return settings.Name },
			want:   current.Name,
		},
		{
			name:   "callbacks: a list replaces the current one",
			update: portals.ClientSettings{Callbacks: []string{"https://acme/other"}},
			field:  "callbacks",
			read:   func(settings portals.ClientSettings) any { return settings.Callbacks },
			want:   []string{"https://acme/other"},
			sent:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (in-memory tenant)", func(t *testing.T) {
			t.Parallel()

			tenant := portalstest.New()
			tenant.AddClient(portals.Client{ID: "cli_1", ClientSettings: current})

			require.NoError(t, tenant.UpdateClient(context.Background(), "cli_1", tt.update))

			client, err := tenant.ReadClient(context.Background(), "cli_1")
			require.NoError(t, err)
			assert.Equal(t, tt.want, tt.read(client.ClientSettings))
		})

		t.Run(tt.name+" (adapter over the SDK)", func(t *testing.T) {
			t.Parallel()

			server, tenant := newPortalsServer(t)
			server.reply(http.MethodPatch, "clients/cli_1", 200, `{"client_id":"cli_1"}`)

			require.NoError(t, tenant.UpdateClient(context.Background(), "cli_1", tt.update))

			body := server.received(http.MethodPatch, "clients/cli_1")[0].json(t)
			value, sent := body[tt.field]
			assert.Equal(t, tt.sent, sent, "the request is %v", body)

			if tt.sent {
				wantJSON, err := json.Marshal(tt.want)
				require.NoError(t, err)
				gotJSON, err := json.Marshal(value)
				require.NoError(t, err)
				assert.JSONEq(t, string(wantJSON), string(gotJSON))
			}
		})
	}
}

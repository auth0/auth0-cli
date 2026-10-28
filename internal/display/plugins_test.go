package display

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/plugins"
)

const (
	// A right to left override, a zero width space and a line separator, which
	// could hide text or end a line in a terminal.
	hostilePluginText = "a\u202eb\u200bc\u2028d"

	// What a terminal gets instead of the hostile text.
	cleanPluginText = "a\ufffdb\ufffdc\ufffdd"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.0 kB"},
		{1536, "1.5 kB"},
		{1_000_000, "1.0 MB"},
		{90_500_000, "90.5 MB"},
		{1_000_000_000, "1.0 GB"},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, formatSize(test.bytes), test.bytes)
	}
}

func TestPluginPermissions_Text(t *testing.T) {
	tests := []struct {
		name        string
		permissions PluginPermissions
		want        string
	}{
		{"none", PluginPermissions{}, "0 scopes, 0 hosts, 0 file paths, 0 environment variables"},
		{"one of each", PluginPermissions{Scopes: 1, Hosts: 1, Files: 1, Env: 1}, "1 scope, 1 host, 1 file path, 1 environment variable"},
		{"several", PluginPermissions{Scopes: 0, Hosts: 2, Files: 3, Env: 4}, "0 scopes, 2 hosts, 3 file paths, 4 environment variables"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.permissions.text())
		})
	}
}

func TestPluginDeprecation_Text(t *testing.T) {
	assert.Equal(t, "Unmaintained", (&PluginDeprecation{Message: "Unmaintained"}).text())
	assert.Equal(t, "Unmaintained (use auth0/other instead)", (&PluginDeprecation{Message: "Unmaintained", Replacement: "auth0/other"}).text())
}

// jsonOf returns the JSON of the object that a view gives for the JSON output.
func jsonOf(t *testing.T, view View) string {
	t.Helper()

	b, err := json.Marshal(view.Object())
	require.NoError(t, err)

	return string(b)
}

func TestPluginSearchView(t *testing.T) {
	tests := []struct {
		name     string
		result   PluginSearchResult
		wantRow  []string
		wantJSON string
	}{
		{
			name:     "empty",
			result:   PluginSearchResult{},
			wantRow:  []string{"", "", "no", ""},
			wantJSON: `{"id":"","name":"","version":"","installed":false,"description":"","deprecated":null}`,
		},
		{
			name:     "installed",
			result:   PluginSearchResult{ID: "auth0/checkmate", Name: "checkmate", Version: "1.2.0", Installed: true, Description: "Audits a tenant"},
			wantRow:  []string{"auth0/checkmate", "1.2.0", "yes", "Audits a tenant"},
			wantJSON: `{"id":"auth0/checkmate","name":"checkmate","version":"1.2.0","installed":true,"description":"Audits a tenant","deprecated":null}`,
		},
		{
			name:     "deprecated with and without a replacement",
			result:   PluginSearchResult{ID: "auth0/old", Name: "old", Version: "1.0.0", Description: "Old", Deprecated: &PluginDeprecation{Message: "Gone", Replacement: "auth0/new"}},
			wantRow:  []string{"auth0/old", "1.0.0", "no", "(deprecated) Old"},
			wantJSON: `{"id":"auth0/old","name":"old","version":"1.0.0","installed":false,"description":"Old","deprecated":{"message":"Gone","replacement":"auth0/new"}}`,
		},
		{
			name:     "deprecated without a replacement",
			result:   PluginSearchResult{ID: "auth0/old", Name: "old", Version: "1.0.0", Deprecated: &PluginDeprecation{Message: "Gone"}},
			wantRow:  []string{"auth0/old", "1.0.0", "no", "(deprecated) "},
			wantJSON: `{"id":"auth0/old","name":"old","version":"1.0.0","installed":false,"description":"","deprecated":{"message":"Gone"}}`,
		},
		{
			name:    "hostile text is cleaned in the row",
			result:  PluginSearchResult{ID: "auth0/" + hostilePluginText, Version: hostilePluginText, Description: hostilePluginText},
			wantRow: []string{"auth0/" + cleanPluginText, cleanPluginText, "no", cleanPluginText},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := &pluginSearchView{test.result}

			assert.Equal(t, []string{"ID", "Version", "Installed", "Description"}, view.AsTableHeader())
			assert.Equal(t, test.wantRow, view.AsTableRow())

			if test.wantJSON != "" {
				assert.JSONEq(t, test.wantJSON, jsonOf(t, view))
			}

			// JSON is data, so it keeps the text as the registry has it.
			var decoded PluginSearchResult
			require.NoError(t, json.Unmarshal([]byte(jsonOf(t, view)), &decoded))
			assert.Equal(t, test.result, decoded)
		})
	}
}

func TestPluginListView(t *testing.T) {
	tests := []struct {
		name     string
		entry    PluginListEntry
		wantRow  []string
		wantJSON string
	}{
		{
			name:     "empty",
			entry:    PluginListEntry{},
			wantRow:  []string{"", "", "no", "", "0 B"},
			wantJSON: `{"id":"","name":"","version":"","pinned":false,"previous":"","sizeBytes":0,"linked":false}`,
		},
		{
			name:     "pinned with a previous version",
			entry:    PluginListEntry{ID: "auth0/checkmate", Name: "checkmate", Version: "2.0.0", Pinned: true, Previous: "1.0.0", SizeBytes: 90_500_000},
			wantRow:  []string{"auth0/checkmate", "2.0.0", "yes", "1.0.0", "90.5 MB"},
			wantJSON: `{"id":"auth0/checkmate","name":"checkmate","version":"2.0.0","pinned":true,"previous":"1.0.0","sizeBytes":90500000,"linked":false}`,
		},
		{
			name:    "hostile text is cleaned in the row",
			entry:   PluginListEntry{ID: "auth0/" + hostilePluginText, Version: hostilePluginText, Previous: hostilePluginText, SizeBytes: 1536},
			wantRow: []string{"auth0/" + cleanPluginText, cleanPluginText, "no", cleanPluginText, "1.5 kB"},
		},
		{
			name:     "a linked plugin",
			entry:    PluginListEntry{ID: "auth0/checkmate", Name: "checkmate", Version: "2.0.0", Linked: true},
			wantRow:  []string{"auth0/checkmate", "2.0.0", "no", "", "0 B"},
			wantJSON: `{"id":"auth0/checkmate","name":"checkmate","version":"2.0.0","pinned":false,"previous":"","sizeBytes":0,"linked":true}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := &pluginListView{PluginListEntry: test.entry}

			assert.Equal(t, []string{"ID", "Version", "Pinned", "Previous", "Size"}, view.AsTableHeader())
			assert.Equal(t, test.wantRow, view.AsTableRow())

			if test.wantJSON != "" {
				assert.JSONEq(t, test.wantJSON, jsonOf(t, view))
			}

			var decoded PluginListEntry
			require.NoError(t, json.Unmarshal([]byte(jsonOf(t, view)), &decoded))
			assert.Equal(t, test.entry, decoded)
		})
	}
}

func TestPluginListView_LinkedColumn(t *testing.T) {
	view := &pluginListView{PluginListEntry: PluginListEntry{ID: "auth0/checkmate", Version: "2.0.0", Linked: true}, withLinked: true}

	assert.Equal(t, []string{"ID", "Version", "Pinned", "Previous", "Size", "Linked"}, view.AsTableHeader())
	assert.Equal(t, []string{"auth0/checkmate", "2.0.0", "no", "", "0 B", "yes"}, view.AsTableRow())
}

func TestPluginChangeView(t *testing.T) {
	tests := []struct {
		name     string
		change   PluginChangeResult
		wantRow  []string
		wantJSON string
	}{
		{
			name:     "empty",
			change:   PluginChangeResult{},
			wantRow:  []string{"", "", ""},
			wantJSON: `{"id":"","version":"","previousVersion":"","action":"","pinned":false,"approvalRequired":false}`,
		},
		{
			name: "an update that needs an approval",
			change: PluginChangeResult{
				ID: "auth0/checkmate", Version: "2.0.0", PreviousVersion: "1.0.0", Action: "installed", ApprovalRequired: true,
				ApproveCommand: "auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com",
			},
			wantRow: []string{"auth0/checkmate", "2.0.0", "installed"},
			wantJSON: `{"id":"auth0/checkmate","version":"2.0.0","previousVersion":"1.0.0","action":"installed","pinned":false,"approvalRequired":true,
				"approveCommand":"auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"}`,
		},
		{
			name:     "a skipped plugin and a failed one",
			change:   PluginChangeResult{ID: "auth0/checkmate", Version: "1.0.0", Action: "skipped", Pinned: true, Available: "2.0.0", Error: "boom"},
			wantRow:  []string{"auth0/checkmate", "1.0.0", "skipped"},
			wantJSON: `{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"skipped","pinned":true,"approvalRequired":false,"available":"2.0.0","error":"boom"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := &pluginChangeView{test.change}

			assert.Equal(t, []string{"Name", "Version", "Action"}, view.AsTableHeader())
			assert.Equal(t, test.wantRow, view.AsTableRow())
			assert.JSONEq(t, test.wantJSON, jsonOf(t, view))
		})
	}
}

func TestPluginInfoView(t *testing.T) {
	approvedAt := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	full := PluginDetails{
		ID:              "auth0/checkmate",
		Version:         "2.0.0",
		Description:     "Audits a tenant",
		Owner:           plugins.Owner{Team: "Tools", Bugs: "https://example.com/bugs"},
		Homepage:        "https://example.com/checkmate",
		Requires:        ">=1.38.0",
		Platforms:       []plugins.Platform{plugins.PlatformDarwinArm64, plugins.PlatformLinuxX64},
		Permissions:     PluginPermissions{Scopes: 2, Hosts: 1},
		PermissionsHash: "sha256:aaa",
		Versions:        []PluginVersion{{Version: "2.0.0", Latest: true}, {Version: "1.1.0", Revoked: true}, {Version: "1.0.0"}},
		Installed:       &PluginInstalled{Version: "1.0.0", Verified: true, PermissionsHash: "sha256:bbb"},
		Deprecated:      &PluginDeprecation{Message: "Unmaintained", Replacement: "auth0/other"},
		ApprovedTenants: []PluginApproval{
			{Tenant: "acme.us.auth0.com", PermissionsHash: "sha256:aaa", ApprovedAt: approvedAt},
			{Tenant: "other.us.auth0.com", PermissionsHash: "sha256:bbb", ApprovedAt: approvedAt},
		},
	}

	tests := []struct {
		name   string
		mutate func(*PluginDetails)
		want   [][]string
	}{
		{
			name: "everything is known",
			want: [][]string{
				{"ID", ansi.Faint("auth0/checkmate")},
				{"VERSION", "2.0.0"},
				{"DESCRIPTION", "Audits a tenant"},
				{"OWNER", "Tools https://example.com/bugs"},
				{"HOMEPAGE", "https://example.com/checkmate"},
				{"REQUIRES", "auth0-cli >=1.38.0"},
				{"PLATFORMS", "darwin-arm64, linux-x64"},
				{"PERMISSIONS", "2 scopes, 1 host, 0 file paths, 0 environment variables"},
				{"PERMISSIONS HASH", "sha256:aaa"},
				{"VERSIONS", "2.0.0 (latest), 1.1.0 (revoked), 1.0.0"},
				{"INSTALLED", "1.0.0"},
				{"DEPRECATED", "Unmaintained (use auth0/other instead)"},
				{"APPROVED TENANTS", "acme.us.auth0.com, other.us.auth0.com (different permissions)"},
			},
		},
		{
			name:   "a linked plugin",
			mutate: func(d *PluginDetails) { d.Linked = true },
			want: [][]string{
				{"ID", ansi.Faint("auth0/checkmate")},
				{"VERSION", "2.0.0"},
				{"DESCRIPTION", "Audits a tenant"},
				{"OWNER", "Tools https://example.com/bugs"},
				{"HOMEPAGE", "https://example.com/checkmate"},
				{"REQUIRES", "auth0-cli >=1.38.0"},
				{"PLATFORMS", "darwin-arm64, linux-x64"},
				{"PERMISSIONS", "2 scopes, 1 host, 0 file paths, 0 environment variables"},
				{"PERMISSIONS HASH", "sha256:aaa"},
				{"VERSIONS", "2.0.0 (latest), 1.1.0 (revoked), 1.0.0"},
				{"INSTALLED", "1.0.0"},
				{"DEPRECATED", "Unmaintained (use auth0/other instead)"},
				{"APPROVED TENANTS", "acme.us.auth0.com, other.us.auth0.com (different permissions)"},
				{"LINKED", "yes, a local build runs in place of any installed copy"},
			},
		},
		{
			name:   "nothing is known",
			mutate: func(d *PluginDetails) { *d = PluginDetails{} },
			want: [][]string{
				{"ID", ansi.Faint("")},
				{"VERSION", ""},
				{"DESCRIPTION", ""},
				{"OWNER", " "},
				{"REQUIRES", "none"},
				{"PLATFORMS", ""},
				{"PERMISSIONS", "0 scopes, 0 hosts, 0 file paths, 0 environment variables"},
				{"PERMISSIONS HASH", ""},
				{"VERSIONS", ""},
				{"INSTALLED", "no"},
				{"APPROVED TENANTS", "none"},
			},
		},
		{
			name: "the latest version is revoked, and the installed copy failed verification",
			mutate: func(d *PluginDetails) {
				d.Versions = []PluginVersion{{Version: "2.0.0", Latest: true, Revoked: true}}
				d.Installed = &PluginInstalled{Version: "2.0.0", Error: "changed"}
				d.Deprecated = &PluginDeprecation{Message: "Unmaintained"}
				d.ApprovedTenants = nil
			},
			want: [][]string{
				{"ID", ansi.Faint("auth0/checkmate")},
				{"VERSION", "2.0.0"},
				{"DESCRIPTION", "Audits a tenant"},
				{"OWNER", "Tools https://example.com/bugs"},
				{"HOMEPAGE", "https://example.com/checkmate"},
				{"REQUIRES", "auth0-cli >=1.38.0"},
				{"PLATFORMS", "darwin-arm64, linux-x64"},
				{"PERMISSIONS", "2 scopes, 1 host, 0 file paths, 0 environment variables"},
				{"PERMISSIONS HASH", "sha256:aaa"},
				{"VERSIONS", "2.0.0 (latest, revoked)"},
				{"INSTALLED", "2.0.0 (failed verification)"},
				{"DEPRECATED", "Unmaintained"},
				{"APPROVED TENANTS", "none"},
			},
		},
		{
			name: "text from the registry is cleaned",
			mutate: func(d *PluginDetails) {
				d.Description = hostilePluginText
				d.Owner = plugins.Owner{Team: hostilePluginText, Bugs: "https://example.com/" + hostilePluginText}
				d.Homepage = "https://example.com/" + hostilePluginText
				d.Requires = hostilePluginText
				d.Deprecated = &PluginDeprecation{Message: hostilePluginText, Replacement: hostilePluginText}
				d.ApprovedTenants = []PluginApproval{{Tenant: hostilePluginText, PermissionsHash: d.PermissionsHash}}
				d.Installed = &PluginInstalled{Version: hostilePluginText, Verified: true}
			},
			want: [][]string{
				{"ID", ansi.Faint("auth0/checkmate")},
				{"VERSION", "2.0.0"},
				{"DESCRIPTION", cleanPluginText},
				{"OWNER", cleanPluginText + " https://example.com/" + cleanPluginText},
				{"HOMEPAGE", "https://example.com/" + cleanPluginText},
				{"REQUIRES", "auth0-cli " + cleanPluginText},
				{"PLATFORMS", "darwin-arm64, linux-x64"},
				{"PERMISSIONS", "2 scopes, 1 host, 0 file paths, 0 environment variables"},
				{"PERMISSIONS HASH", "sha256:aaa"},
				{"VERSIONS", "2.0.0 (latest), 1.1.0 (revoked), 1.0.0"},
				{"INSTALLED", cleanPluginText},
				{"DEPRECATED", cleanPluginText + " (use " + cleanPluginText + " instead)"},
				{"APPROVED TENANTS", cleanPluginText},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			details := full
			if test.mutate != nil {
				test.mutate(&details)
			}

			view := &pluginInfoView{details}

			assert.Equal(t, test.want, view.KeyValues())
			assert.Equal(t, []string{"Name", "Version", "Description"}, view.AsTableHeader())
			assert.Equal(t, []string{details.ID, details.Version, plugins.SafeText(details.Description)}, view.AsTableRow())
		})
	}

	t.Run("the JSON object keeps the text and has every field", func(t *testing.T) {
		details := full
		details.Description = hostilePluginText
		details.Deprecated = &PluginDeprecation{Message: hostilePluginText}

		view := &pluginInfoView{details}

		var decoded PluginDetails
		require.NoError(t, json.Unmarshal([]byte(jsonOf(t, view)), &decoded))
		assert.Equal(t, details, decoded)
	})

	t.Run("the JSON object of an empty view has null and empty lists as the commands print them", func(t *testing.T) {
		view := &pluginInfoView{PluginDetails{Versions: []PluginVersion{}, ApprovedTenants: []PluginApproval{}}}

		assert.JSONEq(t, `{"id":"","version":"","description":"","owner":{"team":"","bugs":""},"homepage":"","requires":"","platforms":null,
			"permissions":{"scopes":0,"hosts":0,"files":0,"env":0},"permissionsHash":"","versions":[],"installed":null,"deprecated":null,"approvedTenants":[],"linked":false}`, jsonOf(t, view))
	})

	t.Run("the JSON object of an approval has its time", func(t *testing.T) {
		view := &pluginInfoView{PluginDetails{ApprovedTenants: []PluginApproval{{Tenant: "acme.us.auth0.com", PermissionsHash: "sha256:aaa", ApprovedAt: approvedAt}}}}

		assert.Contains(t, jsonOf(t, view), `"approvedTenants":[{"tenant":"acme.us.auth0.com","permissionsHash":"sha256:aaa","approvedAt":"2026-10-01T12:00:00Z"}]`)
	})
}

// pluginRenderer returns a renderer that writes to buffers.
func pluginRenderer(format OutputFormat) (*Renderer, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer

	return &Renderer{MessageWriter: &stderr, ResultWriter: &stdout, Format: format}, &stdout, &stderr
}

func TestRenderer_PluginSearchList(t *testing.T) {
	found := []PluginSearchResult{
		{ID: "auth0/checkmate", Name: "checkmate", Version: "1.2.0", Installed: true, Description: "Audits a tenant"},
		{ID: "auth0/old", Name: "old", Version: "1.0.0", Description: hostilePluginText, Deprecated: &PluginDeprecation{Message: "Gone"}},
	}

	t.Run("prints csv", func(t *testing.T) {
		r, stdout, _ := pluginRenderer(OutputFormatCSV)
		r.PluginSearchList(found)

		assert.Equal(t, "ID,Version,Installed,Description\nauth0/checkmate,1.2.0,yes,Audits a tenant\nauth0/old,1.0.0,no,(deprecated) "+cleanPluginText+"\n", stdout.String())
	})

	t.Run("says when there is nothing", func(t *testing.T) {
		r, stdout, stderr := pluginRenderer("")
		r.PluginSearchList(nil)

		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "No plugins available. Try a different search, or run: auth0 plugin search")
	})

	t.Run("prints an empty list as JSON", func(t *testing.T) {
		r, stdout, stderr := pluginRenderer(OutputFormatJSONCompact)
		r.PluginSearchList(nil)

		assert.JSONEq(t, `[]`, stdout.String())
		assert.Empty(t, stderr.String())
	})
}

func TestRenderer_PluginList(t *testing.T) {
	installed := []PluginListEntry{{ID: "auth0/checkmate", Name: "checkmate", Version: "2.0.0", Pinned: true, Previous: "1.0.0", SizeBytes: 1536}}

	t.Run("prints csv", func(t *testing.T) {
		r, stdout, _ := pluginRenderer(OutputFormatCSV)
		r.PluginList(installed)

		assert.Equal(t, "ID,Version,Pinned,Previous,Size\nauth0/checkmate,2.0.0,yes,1.0.0,1.5 kB\n", stdout.String())
	})

	t.Run("has the column of linked plugins only when a plugin is linked", func(t *testing.T) {
		r, stdout, _ := pluginRenderer(OutputFormatCSV)
		r.PluginList(append(slices.Clone(installed), PluginListEntry{ID: "auth0/mine", Version: "0.1.0", Linked: true}))

		assert.Equal(t, "ID,Version,Pinned,Previous,Size,Linked\nauth0/checkmate,2.0.0,yes,1.0.0,1.5 kB,no\nauth0/mine,0.1.0,no,,0 B,yes\n", stdout.String())
	})

	t.Run("says when nothing is installed", func(t *testing.T) {
		r, stdout, stderr := pluginRenderer("")
		r.PluginList(nil)

		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "No installed plugins available. Find one with: auth0 plugin search")
	})
}

func TestRenderer_PluginInfo(t *testing.T) {
	details := PluginDetails{ID: "auth0/checkmate", Version: "1.0.0", Description: hostilePluginText, Versions: []PluginVersion{}, ApprovedTenants: []PluginApproval{}}

	r, stdout, _ := pluginRenderer("")
	r.PluginInfo(details)

	assert.Contains(t, stdout.String(), "DESCRIPTION")
	assert.Contains(t, stdout.String(), cleanPluginText)
	assert.NotContains(t, stdout.String(), "\u202e")
}

func TestRenderer_PluginChange(t *testing.T) {
	change := PluginChangeResult{ID: "auth0/checkmate", Version: "1.0.0", Action: "installed"}

	r, stdout, _ := pluginRenderer(OutputFormatJSONCompact)
	r.PluginChange(change)
	assert.JSONEq(t, `{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"installed","pinned":false,"approvalRequired":false}`, stdout.String())

	r, stdout, _ = pluginRenderer(OutputFormatJSONCompact)
	r.PluginChanges([]PluginChangeResult{change, {ID: "auth0/other", Action: "skipped", Pinned: true, Available: "2.0.0"}})
	assert.JSONEq(t, `[
		{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"installed","pinned":false,"approvalRequired":false},
		{"id":"auth0/other","version":"","previousVersion":"","action":"skipped","pinned":true,"approvalRequired":false,"available":"2.0.0"}
	]`, stdout.String())

	// A person gets the lines that the command printed for each plugin, not a table
	// or a CSV file.
	for _, format := range []OutputFormat{"", OutputFormatCSV} {
		r, stdout, stderr := pluginRenderer(format)
		r.PluginChange(change)
		r.PluginChanges([]PluginChangeResult{change})

		assert.Empty(t, stdout.String(), format)
		assert.Empty(t, stderr.String(), format)
	}
}

func TestRenderer_PluginLines(t *testing.T) {
	r, stdout, stderr := pluginRenderer("")

	r.PluginSummary("auth0/"+hostilePluginText, "1.0.0", hostilePluginText)
	r.PluginDownload(90_500_000, plugins.PlatformLinuxX64)
	r.PluginUninstalled("auth0/checkmate", 1536)

	assert.Empty(t, stdout.String())

	lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
	require.Len(t, lines, 3)
	assert.Contains(t, string(lines[0]), "auth0/"+cleanPluginText+" 1.0.0  "+cleanPluginText)
	assert.NotContains(t, stderr.String(), "\u202e")
	assert.Contains(t, string(lines[1]), "Download: 90.5 MB (linux-x64)")
	assert.Contains(t, string(lines[2]), "Uninstalled auth0/checkmate and freed 1.5 kB.")
}

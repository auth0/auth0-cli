package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/plugins"
)

// The plugin data types below are what the plugin commands hand to the renderer.
// They are exported because the commands build them from the registry, the
// store and the approvals, which the display package does not read, and their
// JSON tags are the JSON output of the commands. They hold the text as the
// registry supplied it. The views that wrap them print it as plain text.

// PluginDeprecation says why a plugin should no longer be used.
type PluginDeprecation struct {
	Message     string `json:"message"`
	Replacement string `json:"replacement,omitempty"`
}

// text returns the deprecation as one sentence for a person.
func (d *PluginDeprecation) text() string {
	if d.Replacement == "" {
		return d.Message
	}

	return fmt.Sprintf("%s (use %s instead)", d.Message, d.Replacement)
}

// PluginSearchResult is one plugin found in the registry.
type PluginSearchResult struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Installed   bool               `json:"installed"`
	Description string             `json:"description"`
	Deprecated  *PluginDeprecation `json:"deprecated"`
}

// PluginListEntry is one installed plugin.
type PluginListEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Pinned    bool   `json:"pinned"`
	Previous  string `json:"previous"`
	SizeBytes int64  `json:"sizeBytes"`
}

// PluginChangeResult is the result of an install or an update of one plugin.
// ApproveCommand is the command that approves the plugin, and is only set when
// ApprovalRequired is true and there is a tenant. Available and Error are only
// set for a plugin that update --all skipped or could not update.
type PluginChangeResult struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	PreviousVersion  string `json:"previousVersion"`
	Action           string `json:"action"`
	Pinned           bool   `json:"pinned"`
	ApprovalRequired bool   `json:"approvalRequired"`
	ApproveCommand   string `json:"approveCommand,omitempty"`
	Available        string `json:"available,omitempty"`
	Error            string `json:"error,omitempty"`
}

// PluginPermissions counts what a plugin version asks to be allowed. The
// consent screen of the install lists the details.
type PluginPermissions struct {
	Scopes int `json:"scopes"`
	Hosts  int `json:"hosts"`
	Files  int `json:"files"`
	Env    int `json:"env"`
}

func (p PluginPermissions) text() string {
	return strings.Join([]string{
		pluralize(p.Scopes, "scope", "scopes"),
		pluralize(p.Hosts, "host", "hosts"),
		pluralize(p.Files, "file path", "file paths"),
		pluralize(p.Env, "environment variable", "environment variables"),
	}, ", ")
}

// PluginVersion is one version of a plugin in the registry.
type PluginVersion struct {
	Version string `json:"version"`
	Latest  bool   `json:"latest"`
	Revoked bool   `json:"revoked"`
}

// PluginInstalled is the installed copy of a plugin. When it failed
// verification, Error says why and nothing else about it is vouched for.
type PluginInstalled struct {
	Version         string `json:"version"`
	Verified        bool   `json:"verified"`
	PermissionsHash string `json:"permissionsHash,omitempty"`
	Error           string `json:"error,omitempty"`
}

// PluginApproval is a tenant where a person approved the plugin.
type PluginApproval struct {
	Tenant          string    `json:"tenant"`
	PermissionsHash string    `json:"permissionsHash"`
	ApprovedAt      time.Time `json:"approvedAt"`
}

// PluginDetails is the detail of one version of a plugin.
type PluginDetails struct {
	ID              string             `json:"id"`
	Version         string             `json:"version"`
	Description     string             `json:"description"`
	Owner           plugins.Owner      `json:"owner"`
	Homepage        string             `json:"homepage"`
	Requires        string             `json:"requires"`
	Platforms       []plugins.Platform `json:"platforms"`
	Permissions     PluginPermissions  `json:"permissions"`
	PermissionsHash string             `json:"permissionsHash"`
	Versions        []PluginVersion    `json:"versions"`
	Installed       *PluginInstalled   `json:"installed"`
	Deprecated      *PluginDeprecation `json:"deprecated"`
	ApprovedTenants []PluginApproval   `json:"approvedTenants"`
}

type pluginSearchView struct {
	PluginSearchResult
}

func (v *pluginSearchView) AsTableHeader() []string {
	return []string{"ID", "Version", "Installed", "Description"}
}

func (v *pluginSearchView) AsTableRow() []string {
	description := v.Description
	if v.Deprecated != nil {
		description = "(deprecated) " + description
	}

	return []string{plugins.SafeText(v.ID), plugins.SafeText(v.Version), yesNo(v.Installed), plugins.SafeText(description)}
}

func (v *pluginSearchView) Object() any {
	return &v.PluginSearchResult
}

type pluginListView struct {
	PluginListEntry
}

func (v *pluginListView) AsTableHeader() []string {
	return []string{"ID", "Version", "Pinned", "Previous", "Size"}
}

func (v *pluginListView) AsTableRow() []string {
	return []string{plugins.SafeText(v.ID), plugins.SafeText(v.Version), yesNo(v.Pinned), plugins.SafeText(v.Previous), formatSize(v.SizeBytes)}
}

func (v *pluginListView) Object() any {
	return &v.PluginListEntry
}

type pluginChangeView struct {
	PluginChangeResult
}

func (v *pluginChangeView) AsTableHeader() []string {
	return []string{"Name", "Version", "Action"}
}

func (v *pluginChangeView) AsTableRow() []string {
	return []string{v.ID, v.Version, v.Action}
}

func (v *pluginChangeView) Object() any {
	return &v.PluginChangeResult
}

type pluginInfoView struct {
	PluginDetails
}

func (v *pluginInfoView) AsTableHeader() []string {
	return []string{"Name", "Version", "Description"}
}

func (v *pluginInfoView) AsTableRow() []string {
	return []string{plugins.SafeText(v.ID), plugins.SafeText(v.Version), plugins.SafeText(v.Description)}
}

func (v *pluginInfoView) KeyValues() [][]string {
	platforms := make([]string, 0, len(v.Platforms))
	for _, platform := range v.Platforms {
		platforms = append(platforms, string(platform))
	}

	versions := make([]string, 0, len(v.Versions))

	for _, version := range v.Versions {
		entry := version.Version

		switch {
		case version.Latest && version.Revoked:
			entry += " (latest, revoked)"
		case version.Latest:
			entry += " (latest)"
		case version.Revoked:
			entry += " (revoked)"
		}

		versions = append(versions, entry)
	}

	requires := "none"
	if v.Requires != "" {
		requires = "auth0-cli " + v.Requires
	}

	installed := "no"

	if v.Installed != nil {
		installed = v.Installed.Version
		if !v.Installed.Verified {
			installed += " (failed verification)"
		}
	}

	rows := [][]string{
		{"ID", ansi.Faint(plugins.SafeText(v.ID))},
		{"VERSION", v.Version},
		{"DESCRIPTION", v.Description},
		{"OWNER", v.Owner.Team + " " + v.Owner.Bugs},
	}

	if v.Homepage != "" {
		rows = append(rows, []string{"HOMEPAGE", v.Homepage})
	}

	rows = append(rows,
		[]string{"REQUIRES", requires},
		[]string{"PLATFORMS", strings.Join(platforms, ", ")},
		[]string{"PERMISSIONS", v.Permissions.text()},
		[]string{"PERMISSIONS HASH", v.PermissionsHash},
		[]string{"VERSIONS", strings.Join(versions, ", ")},
		[]string{"INSTALLED", installed},
	)

	if v.Deprecated != nil {
		rows = append(rows, []string{"DEPRECATED", v.Deprecated.text()})
	}

	approved := "none"

	if len(v.ApprovedTenants) > 0 {
		tenants := make([]string, 0, len(v.ApprovedTenants))

		for _, approval := range v.ApprovedTenants {
			entry := approval.Tenant

			// An approval holds for the permissions it was given for. A tenant that
			// approved other permissions than the ones shown is not approved for them.
			if approval.PermissionsHash != v.PermissionsHash {
				entry += " (different permissions)"
			}

			tenants = append(tenants, entry)
		}

		approved = strings.Join(tenants, ", ")
	}

	rows = append(rows, []string{"APPROVED TENANTS", approved})

	// Everything the registry supplied is printed as plain text. The ID row is
	// already plain, and its color is not text.
	for _, row := range rows[1:] {
		row[1] = plugins.SafeText(row[1])
	}

	return rows
}

func (v *pluginInfoView) Object() any {
	return &v.PluginDetails
}

// PluginSearchList prints the plugins found in the registry.
func (r *Renderer) PluginSearchList(found []PluginSearchResult) {
	if len(found) == 0 {
		r.EmptyState("plugins", "Try a different search, or run: auth0 plugin search")
		return
	}

	results := make([]View, 0, len(found))
	for _, plugin := range found {
		results = append(results, &pluginSearchView{plugin})
	}

	r.Results(results)
}

// PluginList prints the installed plugins.
func (r *Renderer) PluginList(installed []PluginListEntry) {
	if len(installed) == 0 {
		r.EmptyState("installed plugins", "Find one with: auth0 plugin search")
		return
	}

	results := make([]View, 0, len(installed))
	for _, plugin := range installed {
		results = append(results, &pluginListView{plugin})
	}

	r.Results(results)
}

// PluginInfo prints the detail of one version of a plugin.
func (r *Renderer) PluginInfo(details PluginDetails) {
	r.Result(&pluginInfoView{details})
}

// PluginChange prints the result of an install or an update of one plugin as
// JSON. In the other formats it prints nothing, because the command already told
// the person what it did.
func (r *Renderer) PluginChange(change PluginChangeResult) {
	if !r.jsonFormat() {
		return
	}

	r.Result(&pluginChangeView{change})
}

// PluginChanges prints the results of update --all, one entry for each plugin, as
// JSON. In the other formats it prints nothing, for the reason of PluginChange.
func (r *Renderer) PluginChanges(changes []PluginChangeResult) {
	if !r.jsonFormat() {
		return
	}

	results := make([]View, 0, len(changes))
	for _, change := range changes {
		results = append(results, &pluginChangeView{change})
	}

	r.Results(results)
}

func (r *Renderer) jsonFormat() bool {
	return r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact
}

// PluginSummary prints the line that names a plugin: its ID, version and
// description.
func (r *Renderer) PluginSummary(id, version, description string) {
	r.Infof("%s %s  %s", plugins.SafeText(id), plugins.SafeText(version), plugins.SafeText(description))
}

// PluginDownload prints the size of the download of a plugin for a platform.
func (r *Renderer) PluginDownload(bytes int64, platform plugins.Platform) {
	r.Infof("Download: %s (%s)", formatSize(bytes), platform)
}

// PluginUninstalled prints that a plugin was uninstalled and how much space that
// freed.
func (r *Renderer) PluginUninstalled(id string, freedBytes int64) {
	r.Infof("Uninstalled %s and freed %s.", id, formatSize(freedBytes))
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

// formatSize writes a number of bytes the way a person reads a file size.
func formatSize(bytes int64) string {
	const unit = 1000

	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	size, suffix := float64(bytes), 0

	for ; size >= unit && suffix < 3; suffix++ {
		size /= unit
	}

	return fmt.Sprintf("%.1f %s", size, []string{"B", "kB", "MB", "GB"}[suffix])
}

// pluralize writes a count with the singular or the plural of what is counted.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}

	return fmt.Sprintf("%d %s", n, plural)
}

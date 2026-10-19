package display

import (
	"fmt"
	"sort"

	"github.com/auth0/auth0-cli/internal/auth0"
)

const qsBaseURL = "https://auth0.com"

type quickstartView struct {
	Stack   string
	AppType string
	URL     string
	raw     interface{}
}

func (v *quickstartView) AsTableHeader() []string {
	return []string{"Quickstart", "Type", "URL"}
}

func (v *quickstartView) AsTableRow() []string {
	return []string{v.Stack, ApplyColorToFriendlyAppType(v.AppType), v.URL}
}

func (v *quickstartView) Object() interface{} {
	return v.raw
}

func (r *Renderer) QuickstartList(quickstarts []auth0.Quickstart) {
	r.Heading()

	sort.SliceStable(quickstarts, func(i, j int) bool {
		return quickstarts[i].AppType < quickstarts[j].AppType
	})

	var results []View
	for _, qs := range quickstarts {
		results = append(results, &quickstartView{
			Stack:   qs.Name,
			AppType: ApplyColorToFriendlyAppType(qsAppTypeFor(qs.AppType)),
			URL:     fmt.Sprintf("%s%s", qsBaseURL, qs.URL),
			raw:     qs,
		})
	}

	r.Results(results)
}

func qsAppTypeFor(s string) string {
	switch s {
	case "native":
		return friendlyNative
	case "spa":
		return friendlySpa
	case "webapp":
		return friendlyReg
	case "backend":
		return friendlyM2M
	default:
		return ""
	}
}

// QuickstartSetupResult is the single machine-readable result of `auth0 qs setup`.
// Sections that don't apply are null; list fields are always arrays.
type QuickstartSetupResult struct {
	App         *QuickstartSetupApp         `json:"app"`
	API         *QuickstartSetupAPI         `json:"api"`
	ClientGrant *QuickstartSetupClientGrant `json:"client_grant"`
	ConfigFile  *QuickstartSetupConfigFile  `json:"config_file"`
	KeysWritten []string                    `json:"keys_written"` // Names only, never values.
	NextSteps   []string                    `json:"next_steps"`
}

// QuickstartSetupApp describes the application created by `auth0 qs setup`.
type QuickstartSetupApp struct {
	ClientID   string   `json:"client_id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"` // Client app type: spa | regular_web | native | non_interactive.
	Callbacks  []string `json:"callbacks"`
	LogoutURLs []string `json:"logout_urls"`
	WebOrigins []string `json:"web_origins"`
}

// QuickstartSetupAPI describes the API (resource server) created by `auth0 qs setup`.
type QuickstartSetupAPI struct {
	Identifier string   `json:"identifier"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
}

// QuickstartSetupClientGrant describes the client grant linking the app to the API.
type QuickstartSetupClientGrant struct {
	ID       string   `json:"id"`
	ClientID string   `json:"client_id"`
	Audience string   `json:"audience"`
	Scopes   []string `json:"scopes"`
}

// QuickstartSetupConfigFile describes what happened to the generated config file.
type QuickstartSetupConfigFile struct {
	Path   string  `json:"path"`
	Format string  `json:"format"`
	Action string  `json:"action"` // Created | merged | overwritten | skipped.
	Backup *string `json:"backup"` // Null unless overwritten.

	// KeysReplaced lists the keys (names only) whose existing values a merge replaced.
	KeysReplaced []string `json:"keys_replaced"`

	// Notes become next_steps; they are not part of the config_file JSON.
	Notes []string `json:"-"`
}

// QuickstartSetup is a no-op in the human format, whose output is already printed.
func (r *Renderer) QuickstartSetup(res *QuickstartSetupResult) {
	if res == nil {
		return
	}
	if r.Format != OutputFormatJSON && r.Format != OutputFormatJSONCompact {
		return
	}

	res.Normalize()

	if r.Format == OutputFormatJSONCompact {
		r.JSONCompactResult(res)
		return
	}
	r.JSONResult(res)
}

// Normalize turns nil list fields into empty slices so they render as [] instead of null.
func (res *QuickstartSetupResult) Normalize() {
	res.KeysWritten = nonNilStrings(res.KeysWritten)
	res.NextSteps = nonNilStrings(res.NextSteps)
	if res.App != nil {
		res.App.Callbacks = nonNilStrings(res.App.Callbacks)
		res.App.LogoutURLs = nonNilStrings(res.App.LogoutURLs)
		res.App.WebOrigins = nonNilStrings(res.App.WebOrigins)
	}
	if res.API != nil {
		res.API.Scopes = nonNilStrings(res.API.Scopes)
	}
	if res.ClientGrant != nil {
		res.ClientGrant.Scopes = nonNilStrings(res.ClientGrant.Scopes)
	}
	if res.ConfigFile != nil {
		res.ConfigFile.KeysReplaced = nonNilStrings(res.ConfigFile.KeysReplaced)
	}
}

// nonNilStrings keeps JSON list fields as [] instead of null.
func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

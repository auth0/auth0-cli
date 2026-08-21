package display

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/charmbracelet/glamour"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// actionModuleResponse is satisfied by every action module response content
// type returned by the v3 SDK (list item, get, create and update), which all
// share the same getter surface. It lets a single view constructor serve all
// of them.
type actionModuleResponse interface {
	GetID() string
	GetName() string
	GetCode() string
	GetDependencies() []*managementv3.ActionModuleDependency
	GetSecrets() []*managementv3.ActionModuleSecret
	GetActionsUsingModuleTotal() int
	GetAllChangesPublished() bool
	GetLatestVersionNumber() int
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

type actionModuleView struct {
	ID            string
	Name          string
	LatestVersion string
	ActionsUsing  string
	Published     string
	CreatedAt     string
	UpdatedAt     string
	Dependencies  string
	Secrets       string
	Code          string

	raw interface{}
}

func (v *actionModuleView) AsTableHeader() []string {
	return []string{"ID", "Name", "Latest Version", "Actions Using", "Published", "Updated At"}
}

func (v *actionModuleView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Name,
		v.LatestVersion,
		v.ActionsUsing,
		v.Published,
		v.UpdatedAt,
	}
}

func (v *actionModuleView) KeyValues() [][]string {
	keyValues := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"LATEST VERSION", v.LatestVersion},
		{"ACTIONS USING", v.ActionsUsing},
		{"PUBLISHED", v.Published},
		{"CREATED AT", v.CreatedAt},
		{"UPDATED AT", v.UpdatedAt},
	}

	if v.Dependencies != "" {
		keyValues = append(keyValues, []string{"DEPENDENCIES", v.Dependencies})
	}
	if v.Secrets != "" {
		keyValues = append(keyValues, []string{"SECRETS", v.Secrets})
	}

	// CODE is intentionally omitted here; it is rendered as a separate block
	// below the table so multi-line source keeps its formatting instead of
	// being collapsed into a single wrapped cell.

	return keyValues
}

func (v *actionModuleView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ActionModuleList(modules []*managementv3.ActionModuleListItem) {
	resource := "action modules"

	r.Heading(resource)

	if len(modules) == 0 {
		r.EmptyState(resource, "Use 'auth0 actions modules create' to add one")
		return
	}

	var results []View
	for _, m := range modules {
		results = append(results, makeActionModuleView(m))
	}

	r.Results(results)
}

func (r *Renderer) ActionModuleShow(module *managementv3.GetActionModuleResponseContent) {
	r.Heading("action module")
	view := makeActionModuleView(module)
	r.Result(view)
	r.actionModuleCode(view)
}

func (r *Renderer) ActionModuleCreate(module *managementv3.CreateActionModuleResponseContent) {
	r.Heading("action module created")
	view := makeActionModuleView(module)
	r.Result(view)
	r.actionModuleCode(view)
}

func (r *Renderer) ActionModuleUpdate(module *managementv3.UpdateActionModuleResponseContent) {
	r.Heading("action module updated")
	view := makeActionModuleView(module)
	r.Result(view)
	r.actionModuleCode(view)
}

// actionModuleCode prints the module source as its own labelled block beneath
// the key/value table so multi-line code keeps its formatting. The code is
// syntax-highlighted on a styled background via glamour (the same renderer the
// CLI uses for Markdown), falling back to the raw source if rendering fails.
// It is skipped for the JSON output formats, where the code is already part of
// the object.
func (r *Renderer) actionModuleCode(view *actionModuleView) {
	if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		return
	}
	if view.Code == "" {
		return
	}

	fmt.Fprintf(r.ResultWriter, "\n%s\n%s", ansi.Faint("CODE"), highlightJavaScript(view.Code))
}

// highlightJavaScript renders code as a fenced JavaScript block through glamour,
// which syntax-highlights it on a styled background. On any rendering error it
// falls back to the raw code so output is never lost.
func highlightJavaScript(code string) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithPreservedNewLines(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return code + "\n"
	}

	out, err := renderer.Render("```javascript\n" + code + "\n```")
	if err != nil {
		return code + "\n"
	}

	return out
}

func makeActionModuleView(module actionModuleResponse) *actionModuleView {
	// The list endpoint returns the flat latest_version_number field, while the
	// single-item endpoints (get, create, update) instead nest it under
	// latest_version.version_number. Prefer the nested value when present and
	// fall back to the flat one.
	versionNumber := module.GetLatestVersionNumber()
	if m, ok := module.(interface {
		GetLatestVersion() managementv3.ActionModuleVersionReference
	}); ok {
		ref := m.GetLatestVersion()
		if v := ref.GetVersionNumber(); v != 0 {
			versionNumber = v
		}
	}

	latestVersion := "-"
	if versionNumber != 0 {
		latestVersion = "v" + strconv.Itoa(versionNumber)
	}

	return &actionModuleView{
		ID:            module.GetID(),
		Name:          module.GetName(),
		LatestVersion: latestVersion,
		ActionsUsing:  strconv.Itoa(module.GetActionsUsingModuleTotal()),
		Published:     boolean(module.GetAllChangesPublished()),
		CreatedAt:     timeAgo(module.GetCreatedAt()),
		UpdatedAt:     timeAgo(module.GetUpdatedAt()),
		Dependencies:  formatActionModuleDependencies(module.GetDependencies()),
		Secrets:       formatActionModuleSecrets(module.GetSecrets()),
		Code:          module.GetCode(),
		raw:           module,
	}
}

type actionModuleActionView struct {
	ActionID      string
	ActionName    string
	ModuleVersion string

	raw interface{}
}

func (v *actionModuleActionView) AsTableHeader() []string {
	return []string{"Action ID", "Action Name", "Module Version"}
}

func (v *actionModuleActionView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ActionID),
		v.ActionName,
		v.ModuleVersion,
	}
}

func (v *actionModuleActionView) KeyValues() [][]string {
	return [][]string{
		{"ACTION ID", ansi.Faint(v.ActionID)},
		{"ACTION NAME", v.ActionName},
		{"MODULE VERSION", v.ModuleVersion},
	}
}

func (v *actionModuleActionView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ActionModuleActionsList(actions []*managementv3.ActionModuleAction) {
	resource := "actions using module"

	r.Heading(resource)

	if len(actions) == 0 {
		r.EmptyState(resource, "No actions are using this module")
		return
	}

	var results []View
	for _, a := range actions {
		results = append(results, makeActionModuleActionView(a))
	}

	r.Results(results)
}

func makeActionModuleActionView(action *managementv3.ActionModuleAction) *actionModuleActionView {
	number := action.GetModuleVersionNumber()
	version := "-"
	if number != 0 {
		version = "v" + strconv.Itoa(number)
	}

	return &actionModuleActionView{
		ActionID:      action.GetActionID(),
		ActionName:    action.GetActionName(),
		ModuleVersion: version,
		raw:           action,
	}
}

// actionModuleVersionResponse is satisfied by both the version list item and
// the single-version get response, which share the same getter surface. It lets
// one view constructor serve both.
type actionModuleVersionResponse interface {
	GetID() string
	GetModuleID() string
	GetVersionNumber() int
	GetCode() string
	GetDependencies() []*managementv3.ActionModuleDependency
	GetSecrets() []*managementv3.ActionModuleSecret
	GetCreatedAt() time.Time
}

type actionModuleVersionView struct {
	ID           string
	ModuleID     string
	Version      string
	CreatedAt    string
	Dependencies string
	Secrets      string
	Code         string

	raw interface{}
}

func (v *actionModuleVersionView) AsTableHeader() []string {
	return []string{"ID", "Version", "Created At"}
}

func (v *actionModuleVersionView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Version,
		v.CreatedAt,
	}
}

func (v *actionModuleVersionView) KeyValues() [][]string {
	keyValues := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"MODULE ID", ansi.Faint(v.ModuleID)},
		{"VERSION", v.Version},
		{"CREATED AT", v.CreatedAt},
	}

	if v.Dependencies != "" {
		keyValues = append(keyValues, []string{"DEPENDENCIES", v.Dependencies})
	}
	if v.Secrets != "" {
		keyValues = append(keyValues, []string{"SECRETS", v.Secrets})
	}

	// CODE is rendered as a separate block below the table, matching the module
	// show output.

	return keyValues
}

func (v *actionModuleVersionView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ActionModuleVersionList(versions []*managementv3.ActionModuleVersion) {
	resource := "action module versions"

	r.Heading(resource)

	if len(versions) == 0 {
		r.EmptyState(resource, "Publish one with 'auth0 actions modules versions publish'")
		return
	}

	var results []View
	for _, v := range versions {
		results = append(results, makeActionModuleVersionView(v))
	}

	r.Results(results)
}

func (r *Renderer) ActionModuleVersionShow(version *managementv3.GetActionModuleVersionResponseContent) {
	r.Heading("action module version")
	view := makeActionModuleVersionView(version)
	r.Result(view)
	r.actionModuleVersionCode(view)
}

// actionModuleVersionCode prints the version's source as its own labelled block
// beneath the key/value table, matching actionModuleCode.
func (r *Renderer) actionModuleVersionCode(view *actionModuleVersionView) {
	if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		return
	}
	if view.Code == "" {
		return
	}

	fmt.Fprintf(r.ResultWriter, "\n%s\n%s", ansi.Faint("CODE"), highlightJavaScript(view.Code))
}

func makeActionModuleVersionView(version actionModuleVersionResponse) *actionModuleVersionView {
	number := version.GetVersionNumber()
	label := "-"
	if number != 0 {
		label = "v" + strconv.Itoa(number)
	}

	return &actionModuleVersionView{
		ID:           version.GetID(),
		ModuleID:     version.GetModuleID(),
		Version:      label,
		CreatedAt:    timeAgo(version.GetCreatedAt()),
		Dependencies: formatActionModuleDependencies(version.GetDependencies()),
		Secrets:      formatActionModuleSecrets(version.GetSecrets()),
		Code:         version.GetCode(),
		raw:          version,
	}
}

func formatActionModuleDependencies(dependencies []*managementv3.ActionModuleDependency) string {
	if len(dependencies) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(dependencies))
	for _, d := range dependencies {
		formatted = append(formatted, fmt.Sprintf("%s@%s", d.GetName(), d.GetVersion()))
	}

	return strings.Join(formatted, ", ")
}

// formatActionModuleSecrets lists only the secret names. Secret values are
// never returned by the API.
func formatActionModuleSecrets(secrets []*managementv3.ActionModuleSecret) string {
	if len(secrets) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(secrets))
	for _, s := range secrets {
		formatted = append(formatted, s.GetName())
	}

	return strings.Join(formatted, ", ")
}

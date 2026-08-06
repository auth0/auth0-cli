package display

import (
	"fmt"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"golang.org/x/term"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

// clientGrantResponse is satisfied by every client-grant response content type
// returned by the v3 SDK (list, get, create and update), which all share the
// same getter surface. It lets a single view constructor serve all of them.
type clientGrantResponse interface {
	GetID() string
	GetClientID() string
	GetAudience() string
	GetScope() []string
	GetAllowAllScopes() bool
	GetOrganizationUsage() managementv3.ClientGrantOrganizationUsageEnum
	GetAllowAnyOrganization() bool
	GetDefaultFor() managementv3.ClientGrantDefaultForEnum
	GetIsSystem() bool
	GetSubjectType() managementv3.ClientGrantSubjectTypeEnum
	GetAuthorizationDetailsTypes() []string
}

// clientGrantView renders a single client grant as a key-value detail view
// (show, create and update). It carries the full scope list because the user
// asked for that one grant specifically.
type clientGrantView struct {
	ID                        string
	ClientID                  string
	Audience                  string
	Scopes                    string
	SubjectType               string
	OrganizationUsage         string
	AllowAnyOrganization      string
	AuthorizationDetailsTypes string

	raw interface{}
}

func (v *clientGrantView) AsTableHeader() []string {
	return []string{}
}

func (v *clientGrantView) AsTableRow() []string {
	return []string{}
}

func (v *clientGrantView) KeyValues() [][]string {
	keyValues := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"CLIENT ID", v.ClientID},
		{"AUDIENCE", v.Audience},
		{"SCOPES", v.Scopes},
		{"SUBJECT TYPE", v.SubjectType},
	}

	// Only show the organization rows when the grant actually uses
	// organizations. With no organization usage, allow_any_organization is
	// always false, so both rows would just be noise.
	if v.OrganizationUsage != "" {
		keyValues = append(keyValues,
			[]string{"ORGANIZATION USAGE", v.OrganizationUsage},
			[]string{"ALLOW ANY ORGANIZATION", v.AllowAnyOrganization},
		)
	}

	// Only show the authorization_details types when the grant carries any,
	// since most grants do not use Rich Authorization Requests and an empty
	// row would just be noise.
	if v.AuthorizationDetailsTypes != "" {
		keyValues = append(keyValues,
			[]string{"AUTHORIZATION DETAILS TYPES", v.AuthorizationDetailsTypes},
		)
	}

	return keyValues
}

func (v *clientGrantView) Object() interface{} {
	return v.raw
}

// clientGrantTableView renders a client grant as a single row in the list
// table. It shows the scope count rather than the scope values, because
// dumping every scope inline pads the whole column to the widest grant and
// blows the table up (a single grant can carry hundreds of scopes). A grant
// that allows all scopes shows "all" instead of a count, since its scope list
// is empty and a bare 0 would misleadingly read as no access.
type clientGrantTableView struct {
	ID       string
	ClientID string
	Audience string
	Scopes   string

	raw interface{}
}

func (v *clientGrantTableView) AsTableHeader() []string {
	return []string{"ID", "Client ID", "Audience", "Scopes"}
}

func (v *clientGrantTableView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.ClientID, v.Audience, v.Scopes}
}

func (v *clientGrantTableView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ClientGrantList(grants []*managementv3.ClientGrantResponseContent) {
	resource := "client grants"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(grants)))

	if len(grants) == 0 {
		r.EmptyState(resource, "Use 'auth0 client-grants create' to add one")
		return
	}

	var results []View
	for _, grant := range grants {
		results = append(results, makeClientGrantTableView(grant))
	}

	r.Results(results)
}

func (r *Renderer) ClientGrantShow(grant *managementv3.GetClientGrantResponseContent) {
	r.Heading("client grant")
	view, truncated := makeClientGrantView(grant)
	r.Result(view)
	r.hintClientGrantValuesTruncated(grant.GetID(), truncated)
}

func (r *Renderer) ClientGrantCreate(grant *managementv3.CreateClientGrantResponseContent) {
	r.Heading("client grant created")
	view, truncated := makeClientGrantView(grant)
	r.Result(view)
	r.hintClientGrantValuesTruncated(grant.GetID(), truncated)
}

func (r *Renderer) ClientGrantUpdate(grant *managementv3.UpdateClientGrantResponseContent) {
	r.Heading("client grant updated")
	view, truncated := makeClientGrantView(grant)
	r.Result(view)
	r.hintClientGrantValuesTruncated(grant.GetID(), truncated)
}

func (r *Renderer) hintClientGrantValuesTruncated(id string, truncated bool) {
	if !truncated || r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		return
	}
	r.Newline()
	r.Infof("Some values were truncated for display. To see the full list, run %s", ansi.Faint(fmt.Sprintf("client-grants show %s --json", id)))
}

func makeClientGrantView(grant clientGrantResponse) (*clientGrantView, bool) {
	scopes, scopesTruncated := clientGrantValuesForDisplay(grant.GetScope())

	// A grant with allow_all_scopes carries no explicit scope list, so show
	// that it authorizes everything rather than rendering a blank field.
	if grant.GetAllowAllScopes() {
		scopes, scopesTruncated = "(all scopes)", false
	}

	// A grant with no explicit subject type is a client grant, so default the
	// display to "client" rather than leaving the field blank.
	subjectType := string(grant.GetSubjectType())
	if subjectType == "" {
		subjectType = "client"
	}

	// Authorization details types can be a long list too, so truncate them the
	// same way as scopes rather than blowing the value column up.
	authorizationDetailsTypes, authDetailsTruncated := clientGrantValuesForDisplay(grant.GetAuthorizationDetailsTypes())

	view := &clientGrantView{
		ID:                        grant.GetID(),
		ClientID:                  clientGrantIdentifier(grant),
		Audience:                  grant.GetAudience(),
		Scopes:                    scopes,
		SubjectType:               subjectType,
		OrganizationUsage:         string(grant.GetOrganizationUsage()),
		AllowAnyOrganization:      boolean(grant.GetAllowAnyOrganization()),
		AuthorizationDetailsTypes: authorizationDetailsTypes,
		raw:                       grant,
	}
	return view, scopesTruncated || authDetailsTruncated
}

func makeClientGrantTableView(grant clientGrantResponse) *clientGrantTableView {
	scopes := fmt.Sprint(len(grant.GetScope()))
	if grant.GetAllowAllScopes() {
		scopes = "all"
	}

	return &clientGrantTableView{
		ID:       grant.GetID(),
		ClientID: clientGrantIdentifier(grant),
		Audience: grant.GetAudience(),
		Scopes:   scopes,
		raw:      grant,
	}
}

// clientGrantIdentifier returns the client id of a grant, falling back to its
// default_for value for system grants that have no explicit client.
func clientGrantIdentifier(grant clientGrantResponse) string {
	if clientID := grant.GetClientID(); clientID != "" {
		return clientID
	}
	return string(grant.GetDefaultFor())
}

// clientGrantValuesForDisplay joins a list of values (scopes or authorization
// details types) into a single line for the detail view, truncating to the
// terminal width so a grant with hundreds of values does not blow the value
// column up. It returns the display string and whether truncation happened.
func clientGrantValuesForDisplay(values []string) (string, bool) {
	const (
		ellipsis  = "..."
		separator = ", "
		padding   = 32 // The longest clientGrantView key plus surrounding spaces in the label column.
	)

	terminalWidth, _, err := term.GetSize(int(iostream.Input.Fd()))
	if err != nil {
		terminalWidth = 80
	}

	joined := strings.Join(values, separator)
	maxCharacters := terminalWidth - padding

	if len(joined) <= maxCharacters {
		return joined, false
	}

	truncationIndex := maxCharacters - len(ellipsis)
	if truncationIndex < 0 {
		truncationIndex = 0
	}
	if lastSeparator := strings.LastIndex(joined[:truncationIndex], separator); lastSeparator != -1 {
		truncationIndex = lastSeparator
	}

	return joined[:truncationIndex] + ellipsis, true
}

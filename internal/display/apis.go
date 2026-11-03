package display

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"golang.org/x/term"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

type apiView struct {
	ID                  string
	Name                string
	Identifier          string
	Scopes              string
	TokenLifetime       int
	OfflineAccess       string
	SigningAlgorithm    string
	EnforcePolicies     string
	TokenDialect        string
	SubjectTypeAuthJSON string
	ClientID            string

	raw interface{}
}

func (v *apiView) AsTableHeader() []string {
	return []string{}
}

func (v *apiView) AsTableRow() []string {
	return []string{}
}

func (v *apiView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"IDENTIFIER", v.Identifier},
		{"SCOPES", v.Scopes},
		{"TOKEN LIFETIME", strconv.Itoa(v.TokenLifetime)},
		{"ALLOW OFFLINE ACCESS", v.OfflineAccess},
		{"SIGNING ALGORITHM", v.SigningAlgorithm},
		{"ENFORCE POLICIES", v.EnforcePolicies},
	}

	if len(v.TokenDialect) > 0 {
		kvs = append(kvs, []string{"TOKEN DIALECT", v.TokenDialect})
	}

	if len(v.SubjectTypeAuthJSON) > 0 {
		kvs = append(kvs, []string{"SUBJECT TYPE AUTHORIZATION", v.SubjectTypeAuthJSON})
	}

	if len(v.ClientID) > 0 {
		kvs = append(kvs, []string{"CLIENT ID", v.ClientID})
	}

	return kvs
}

func (v *apiView) Object() interface{} {
	return v.raw
}

type apiTableColumn struct{ field, header string }

// apiTableColumns are the table columns, keyed by their API field name.
var apiTableColumns = []apiTableColumn{
	{"id", "ID"},
	{"name", "Name"},
	{"identifier", "Identifier"},
	{"scopes", "Scopes"},
}

type apiTableView struct {
	ID         string
	Name       string
	Identifier string
	Scopes     int

	// Fields limits the columns to these API fields; empty shows every column.
	fields []string
	raw    interface{}
}

func (v *apiTableView) AsTableHeader() []string {
	var header []string
	for _, column := range apiTableColumns {
		if v.shows(column.field) {
			header = append(header, column.header)
		}
	}

	return header
}

func (v *apiTableView) AsTableRow() []string {
	values := map[string]string{
		"id":         ansi.Faint(v.ID),
		"name":       v.Name,
		"identifier": v.Identifier,
		"scopes":     fmt.Sprint(v.Scopes),
	}

	var row []string
	for _, column := range apiTableColumns {
		if v.shows(column.field) {
			row = append(row, values[column.field])
		}
	}

	return row
}

func (v *apiTableView) shows(field string) bool {
	return len(v.fields) == 0 || slices.Contains(v.fields, field)
}

func (v *apiTableView) Object() interface{} {
	return v.raw
}

func (r *Renderer) APIList(apis []*management.ResourceServer) {
	views := make([]*apiTableView, 0, len(apis))
	for _, api := range apis {
		views = append(views, makeAPITableView(api))
	}

	r.apiTableResults(views, "Use 'auth0 apis create' to add one")
}

// APISearchColumns returns the table columns left in the response by the --fields list
// (or, with exclude, by --exclude-fields). Empty fields keeps every column.
func APISearchColumns(fields []string, exclude bool) []string {
	var columns []string
	for _, column := range apiTableColumns {
		if len(fields) == 0 || slices.Contains(fields, column.field) != exclude {
			columns = append(columns, column.field)
		}
	}

	return columns
}

// APISearchList renders search results, limiting the table to the columns left by fields.
func (r *Renderer) APISearchList(apis []*managementv3.ResourceServerSearchResponse, fields []string, exclude bool) {
	columns := APISearchColumns(fields, exclude)

	if len(columns) == 0 && len(apis) > 0 && r.Format == "" {
		r.Infof("No table columns (id, name, identifier, scopes) are left in the response; showing JSON instead.")
		r.JSONResult(apis)
		return
	}

	views := make([]*apiTableView, 0, len(apis))
	for _, api := range apis {
		views = append(views, &apiTableView{
			ID:         api.GetID(),
			Name:       api.GetName(),
			Identifier: api.GetIdentifier(),
			Scopes:     len(api.GetScopes()),

			fields: columns,
			raw:    api,
		})
	}

	r.apiTableResults(views, "Try a broader --filter; results may lag very recent writes")
}

func (r *Renderer) apiTableResults(views []*apiTableView, emptyHint string) {
	resource := "apis"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(views)))

	if len(views) == 0 {
		r.EmptyState(resource, emptyHint)
		return
	}

	results := make([]View, 0, len(views))
	for _, view := range views {
		results = append(results, view)
	}

	r.Results(results)
}

func (r *Renderer) APIShow(api *management.ResourceServer, jsonFlag bool) {
	r.Heading("api")
	view, scopesTruncated := makeAPIView(api)
	r.Result(view)
	if scopesTruncated && !jsonFlag {
		r.Newline()
		r.Infof("Scopes truncated for display. To see the full list, run %s", ansi.Faint(fmt.Sprintf("apis scopes list %s", *api.ID)))
	}
}

func (r *Renderer) APICreate(api *management.ResourceServer) {
	r.Heading("api created")
	view, _ := makeAPIView(api)
	r.Result(view)
}

func (r *Renderer) APIUpdate(api *management.ResourceServer) {
	r.Heading("api updated")
	view, _ := makeAPIView(api)
	r.Result(view)
}

func makeAPIView(api *management.ResourceServer) (*apiView, bool) {
	scopes, scopesTruncated := getScopes(api.GetScopes())

	var subjectTypeAuthJSON string
	if api.SubjectTypeAuthorization != nil {
		if subjectTypeAuthString, err := toJSONString(api.SubjectTypeAuthorization); err == nil {
			subjectTypeAuthJSON = subjectTypeAuthString
		}
	}

	view := &apiView{
		ID:                  ansi.Faint(api.GetID()),
		Name:                api.GetName(),
		Identifier:          api.GetIdentifier(),
		Scopes:              scopes,
		TokenLifetime:       api.GetTokenLifetime(),
		OfflineAccess:       boolean(api.GetAllowOfflineAccess()),
		SigningAlgorithm:    api.GetSigningAlgorithm(),
		EnforcePolicies:     boolean(api.GetEnforcePolicies()),
		TokenDialect:        api.GetTokenDialect(),
		SubjectTypeAuthJSON: subjectTypeAuthJSON,
		ClientID:            api.GetClientID(),
		raw:                 api,
	}
	return view, scopesTruncated
}

func makeAPITableView(api *management.ResourceServer) *apiTableView {
	scopes := len(api.GetScopes())

	return &apiTableView{
		ID:         ansi.Faint(api.GetID()),
		Name:       api.GetName(),
		Identifier: api.GetIdentifier(),
		Scopes:     scopes,

		raw: api,
	}
}

type scopeView struct {
	Scope       string
	Description string
	raw         interface{}
}

func (v *scopeView) AsTableHeader() []string {
	return []string{"Scope", "Description"}
}

func (v *scopeView) AsTableRow() []string {
	return []string{v.Scope, v.Description}
}

func (v *scopeView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ScopesList(api string, scopes []management.ResourceServerScope) {
	resource := "scopes"

	r.Heading(fmt.Sprintf("%s of %s", resource, ansi.Bold(api)))

	if len(scopes) == 0 {
		r.EmptyState(resource, "")
		return
	}

	var results []View
	for _, scope := range scopes {
		results = append(results, makeScopeView(scope))
	}

	r.Results(results)
}

func makeScopeView(scope management.ResourceServerScope) *scopeView {
	return &scopeView{
		Scope:       scope.GetValue(),
		Description: scope.GetDescription(),
		raw:         scope,
	}
}

func getScopes(scopes []management.ResourceServerScope) (string, bool) {
	ellipsis := "..."
	separator := " "
	padding := 22 // The longest apiView key plus two spaces before and after in the label column.
	terminalWidth, _, err := term.GetSize(int(iostream.Input.Fd()))
	if err != nil {
		terminalWidth = 80
	}

	var scopesForDisplay string
	maxCharacters := terminalWidth - padding

	for i, scope := range scopes {
		prepend := separator

		// No separator prepended for first value.
		if i == 0 {
			prepend = ""
		}
		scopesForDisplay += fmt.Sprintf("%s%s", prepend, scope.GetValue())
	}

	if len(scopesForDisplay) <= maxCharacters {
		return scopesForDisplay, false
	}

	truncationIndex := maxCharacters - len(ellipsis)
	lastSeparator := strings.LastIndex(scopesForDisplay[:truncationIndex], separator)
	if lastSeparator != -1 {
		truncationIndex = lastSeparator
	}

	scopesForDisplay = fmt.Sprintf("%s%s", scopesForDisplay[:truncationIndex], ellipsis)

	return scopesForDisplay, true
}

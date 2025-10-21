package display

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
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

type apiTableView struct {
	ID         string
	Name       string
	Identifier string
	Scopes     int

	raw interface{}
}

func (v *apiTableView) AsTableHeader() []string {
	return []string{"ID", "Name", "Identifier", "Scopes"}
}

func (v *apiTableView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Identifier, fmt.Sprint(v.Scopes)}
}

func (v *apiTableView) Object() interface{} {
	return v.raw
}

func (r *Renderer) APIList(apis []*management.ResourceServer) {
	resource := "apis"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(apis)))

	if len(apis) == 0 {
		r.EmptyState(resource, "Use 'auth0 apis create' to add one")
		return
	}

	results := []View{}

	for _, api := range apis {
		results = append(results, makeAPITableView(api))
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

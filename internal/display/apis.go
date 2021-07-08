package display

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"golang.org/x/term"
	"gopkg.in/auth0.v5/management"
)

type apiView struct {
	ID            string
	Name          string
	Identifier    string
	Scopes        string
	TokenLifetime int
	OfflineAccess string

	raw interface{}
}

func (v *apiView) AsTableHeader() []string {
	return []string{}
}

func (v *apiView) AsTableRow() []string {
	return []string{}
}

func (v *apiView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"IDENTIFIER", v.Identifier},
		{"SCOPES", v.Scopes},
		{"TOKEN LIFETIME", strconv.Itoa(v.TokenLifetime)},
		{"ALLOW OFFLINE ACCESS", v.OfflineAccess},
	}
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

func (r *Renderer) ApiList(apis []*management.ResourceServer) {
	resource := "APIs"

	r.Heading(resource)

	if len(apis) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 apis create' to add one")
		return
	}

	results := []View{}

	for _, api := range apis {
		results = append(results, makeApiTableView(api))
	}

	r.Results(results)
}

func (r *Renderer) ApiShow(api *management.ResourceServer) {
	r.Heading("API")
	view, scopesTruncated := makeApiView(api)
	r.Result(view)
	if scopesTruncated {
		r.Newline()
		r.Infof("Scopes truncated for display. To see the full list, run %s", ansi.Faint(fmt.Sprintf("apis scopes list %s", *api.ID)))
	}
}

func (r *Renderer) ApiCreate(api *management.ResourceServer) {
	r.Heading("API created")
	view, _ := makeApiView(api)
	r.Result(view)
}

func (r *Renderer) ApiUpdate(api *management.ResourceServer) {
	r.Heading("API updated")
	view, _ := makeApiView(api)
	r.Result(view)
}

func makeApiView(api *management.ResourceServer) (*apiView, bool) {
	scopes, scopesTruncated := getScopes(api.Scopes)
	view := &apiView{
		ID:            ansi.Faint(api.GetID()),
		Name:          api.GetName(),
		Identifier:    api.GetIdentifier(),
		Scopes:        scopes,
		TokenLifetime: api.GetTokenLifetime(),
		OfflineAccess: boolean(api.GetAllowOfflineAccess()),

		raw: api,
	}
	return view, scopesTruncated
}

func makeApiTableView(api *management.ResourceServer) *apiTableView {
	scopes := len(api.Scopes)

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

func (r *Renderer) ScopesList(api string, scopes []*management.ResourceServerScope) {
	resource := "scopes"

	r.Heading(fmt.Sprintf("%s of %s", resource, ansi.Bold(api)))

	if len(scopes) == 0 {
		r.EmptyState(resource)
		return
	}

	results := []View{}

	for _, scope := range scopes {
		results = append(results, makeScopeView(scope))
	}

	r.Results(results)
}

func makeScopeView(scope *management.ResourceServerScope) *scopeView {
	return &scopeView{
		Scope:       auth0.StringValue(scope.Value),
		Description: auth0.StringValue(scope.Description),
		raw:         scope,
	}
}

func getScopes(scopes []*management.ResourceServerScope) (string, bool) {
	ellipsis := "..."
	separator := " "
	padding := 22 // the longest apiView key plus two spaces before and after in the label column
	terminalWidth, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		terminalWidth = 80
	}

	var scopesForDisplay string
	maxCharacters := terminalWidth - padding

	for i, scope := range scopes {
		prepend := separator

		// no separator prepended for first value
		if i == 0 {
			prepend = ""
		}
		scopesForDisplay += fmt.Sprintf("%s%s", prepend, *scope.Value)
	}

	if len(scopesForDisplay) <= maxCharacters {
		return scopesForDisplay, false
	}

	truncationIndex := maxCharacters - len(ellipsis)
	lastSeparator := strings.LastIndex(string(scopesForDisplay[:truncationIndex]), separator)
	if lastSeparator != -1 {
		truncationIndex = lastSeparator
	}

	scopesForDisplay = fmt.Sprintf("%s%s", string(scopesForDisplay[:truncationIndex]), ellipsis)

	return scopesForDisplay, true
}

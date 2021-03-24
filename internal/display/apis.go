package display

import (
	"fmt"
	"os"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"golang.org/x/term"
	"gopkg.in/auth0.v5/management"
)

type apiView struct {
	ID         string
	Name       string
	Identifier string
	Scopes     string

	raw interface{}
}

func (v *apiView) AsTableHeader() []string {
	return []string{"ID", "Name", "Identifier", "Scopes"}
}

func (v *apiView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Identifier, fmt.Sprint(v.Scopes)}
}

func (v *apiView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"IDENTIFIER", v.Identifier},
		{"SCOPES", v.Scopes},
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
	r.Heading(ansi.Bold(r.Tenant), "APIs\n")

	results := []View{}

	for _, api := range apis {
		results = append(results, makeApiTableView(api))
	}

	r.Results(results)
}

func (r *Renderer) ApiShow(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API\n")
	view, scopesTruncated := makeApiView(api)
	r.Result(view)
	if scopesTruncated {
		r.Newline()
		r.Infof("Scopes truncated for display. To see the full list, run %s", ansi.Faint(fmt.Sprintf("apis scopes list %s", *api.ID)))
	}
}

func (r *Renderer) ApiCreate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API created\n")
	view, _ := makeApiView(api)
	r.Result(view)
}

func (r *Renderer) ApiUpdate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API updated\n")
	view, _ := makeApiView(api)
	r.Result(view)
}

func makeApiView(api *management.ResourceServer) (*apiView, bool) {
	scopes, scopesTruncated := getScopes(api.Scopes)
	view := &apiView{
		ID:         auth0.StringValue(api.ID),
		Name:       auth0.StringValue(api.Name),
		Identifier: auth0.StringValue(api.Identifier),
		Scopes:     auth0.StringValue(scopes),

		raw: api,
	}
	return view, scopesTruncated
}

func makeApiTableView(api *management.ResourceServer) *apiTableView {
	scopes := len(api.Scopes)

	return &apiTableView{
		ID:         auth0.StringValue(api.ID),
		Name:       auth0.StringValue(api.Name),
		Identifier: auth0.StringValue(api.Identifier),
		Scopes:     auth0.IntValue(&scopes),

		raw: api,
	}
}

type scopeView struct {
	Scope       string
	Description string
}

func (v *scopeView) AsTableHeader() []string {
	return []string{"Scope", "Description"}
}

func (v *scopeView) AsTableRow() []string {
	return []string{v.Scope, v.Description}
}

func (r *Renderer) ScopesList(api string, scopes []*management.ResourceServerScope) {
	r.Heading(ansi.Bold(r.Tenant), fmt.Sprintf("Scopes of %s\n", ansi.Bold(api)))

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
	}
}

func getScopes(scopes []*management.ResourceServerScope) (*string, bool) {
	ellipsis := "..."
	separator := " "
	padding := 16 // the longest apiView key plus two spaces before and after in the label column
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
		return &scopesForDisplay, false
	}

	truncationIndex := maxCharacters - len(ellipsis)
	lastSeparator := strings.LastIndex(string(scopesForDisplay[:truncationIndex]), separator)
	if lastSeparator != -1 {
		truncationIndex = lastSeparator
	}

	scopesForDisplay = fmt.Sprintf("%s%s", string(scopesForDisplay[:truncationIndex]), ellipsis)

	return &scopesForDisplay, true
}

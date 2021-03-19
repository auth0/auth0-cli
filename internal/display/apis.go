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

func (r *Renderer) ApiList(apis []*management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "APIs\n")

	results := []View{}

	for _, api := range apis {
		results = append(results, makeApiView(api))
	}

	r.Results(results)
}

func (r *Renderer) ApiShow(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API\n")
	r.Result(makeApiView(api))
}

func (r *Renderer) ApiCreate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API created\n")
	r.Result(makeApiView(api))
}

func (r *Renderer) ApiUpdate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API updated\n")
	r.Result(makeApiView(api))
}

func makeApiView(api *management.ResourceServer) *apiView {

	return &apiView{
		ID:         auth0.StringValue(api.ID),
		Name:       auth0.StringValue(api.Name),
		Identifier: auth0.StringValue(api.Identifier),
		Scopes:     auth0.StringValue(truncateScopes(api.Scopes)),

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

func truncateScopes(scopes []*management.ResourceServerScope) *string {
	ellipsis := "..."
	padding := 16 // the longest apiView key plus two spaces before and after in the label column
	width, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		width = 80
	}

	var results []string
	charactersNeeded := width - len(ellipsis) - padding

	for _, scope := range scopes {
		runeScope := []rune(*scope.Value)
		characterLength := len(runeScope)
		if charactersNeeded < characterLength {
			break
		}
		charactersNeeded -= characterLength + 1 // add one for the space between them
		results = append(results, *scope.Value)
	}

	var truncatedScopes string

	if len(results) == 0 {
		truncatedScopes = "n/a"
	} else {
		truncatedScopes = fmt.Sprintf("%s...", strings.NewReplacer("[", "", "]", "").Replace(fmt.Sprintf("%v", results)))
	}

	return &truncatedScopes
}

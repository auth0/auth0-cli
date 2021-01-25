package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type actionView struct {
	Name      string
	CreatedAt string
}

func (v *actionView) AsTableHeader() []string {
	return []string{"Name", "CreatedAt"}
}

func (v *actionView) AsTableRow() []string {
	return []string{v.Name, v.CreatedAt}
}

func (r *Renderer) ActionList(actions []*management.Action) {
	r.Heading(ansi.Bold(r.Tenant), "actions\n")

	var res []View
	for _, a := range actions {
		res = append(res, &actionView{
			Name:      auth0.StringValue(a.Name),
			CreatedAt: a.CreatedAt.String(),
			// Type:    auth0.StringValue(a.SupportedTriggers[0]),
			// Runtime: auth0.StringValue(a.Runtime),
		})

	}

	r.Results(res)
}

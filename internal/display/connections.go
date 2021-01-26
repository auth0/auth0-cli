package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type connectionView struct {
	Name         string
	Strategy     string
	ConnectionID string
}

func (v *connectionView) AsTableHeader() []string {
	return []string{"Name", "Type", "Connection ID"}
}

func (v *connectionView) AsTableRow() []string {
	return []string{v.Name, v.Strategy, ansi.Faint(v.ConnectionID)}
}

func (r *Renderer) ConnectionList(connections []*management.Connection) {
	r.Heading(ansi.Bold(r.Tenant), "connections\n")

	var res []View
	for _, c := range connections {
		res = append(res, &connectionView{
			Name:         auth0.StringValue(c.Name),
			Strategy:     auth0.StringValue(c.Strategy),
			ConnectionID: auth0.StringValue(c.ID),
		})

	}

	r.Results(res)
}

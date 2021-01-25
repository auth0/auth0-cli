package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type apiView struct {
	ID         string
	Name       string
	Identifier string
}

func (v *apiView) AsTableHeader() []string {
	return []string{"ID", "Name", "Identifier"}
}

func (v *apiView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Identifier}
}

func (r *Renderer) ApisList(apis []*management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "APIs\n")

	var res []View

	for _, api := range apis {
		res = append(res, &apiView{
			ID:         auth0.StringValue(api.ID),
			Name:       auth0.StringValue(api.Name),
			Identifier: auth0.StringValue(api.Identifier),
		})
	}

	r.Results(res)
}

func (r *Renderer) ApiCreate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API created\n")

	v := &apiView{
		ID:         auth0.StringValue(api.ID),
		Name:       auth0.StringValue(api.Name),
		Identifier: auth0.StringValue(api.Identifier),
	}

	r.Results([]View{v})
}

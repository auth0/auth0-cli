package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
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

func (r *Renderer) ApiList(apis []*management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "APIs\n")

	var res []View

	for _, api := range apis {
		res = append(res, makeView(api))
	}

	r.Results(res)
}

func (r *Renderer) ApiCreate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API created\n")
	r.Results([]View{makeView(api)})
}

func (r *Renderer) ApiUpdate(api *management.ResourceServer) {
	r.Heading(ansi.Bold(r.Tenant), "API updated\n")
	r.Results([]View{makeView(api)})
}

func makeView(api *management.ResourceServer) *apiView {
	return &apiView{
		ID:         auth0.StringValue(api.ID),
		Name:       auth0.StringValue(api.Name),
		Identifier: auth0.StringValue(api.Identifier),
	}
}

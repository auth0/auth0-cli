package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type roleView struct {
	Name        string
	ID          string
	Description string
}

func (v *roleView) AsTableHeader() []string {
	return []string{"Name", "Role ID", "Description"}
}

func (v *roleView) AsTableRow() []string {
	return []string{
		v.Name,
		ansi.Faint(v.ID),
		v.Description,
	}
}

func (r *Renderer) RoleList(roles []*management.Role) {
	resource := "roles"

	r.Heading(resource)

	if len(roles) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 roles create' to add one")
		return
	}

	var res []View
	for _, role := range roles {
		res = append(res, &roleView{
			Name:        role.GetName(),
			ID:          role.GetID(),
			Description: role.GetDescription(),
		})
	}

	r.Results(res)
}

func (r *Renderer) RoleGet(role *management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "role\n")
	r.Results([]View{&roleView{
		Name:        auth0.StringValue(role.Name),
		ID:          auth0.StringValue(role.ID),
		Description: auth0.StringValue(role.Description),
	}})
}

func (r *Renderer) RoleUpdate(role *management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "role\n")
	r.Results([]View{&roleView{
		Name:        auth0.StringValue(role.Name),
		ID:          auth0.StringValue(role.ID),
		Description: auth0.StringValue(role.Description),
	}})
}

func (r *Renderer) RoleCreate(role *management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "role\n")
	r.Results([]View{&roleView{
		Name:        auth0.StringValue(role.Name),
		ID:          auth0.StringValue(role.ID),
		Description: auth0.StringValue(role.Description),
	}})
}

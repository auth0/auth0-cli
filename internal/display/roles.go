package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type roleView struct {
	ID          string
	Name        string
	Description string
}

func (v *roleView) AsTableHeader() []string {
	return []string{"ID", "Name", "Description"}
}

func (v *roleView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Name,
		v.Description,
	}
}

func (v *roleView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"DESCRIPTION", v.Description},
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

func (r *Renderer) RoleShow(role *management.Role) {
	r.Heading("role")
	r.roleResult(role)
}

func (r *Renderer) RoleCreate(role *management.Role) {
	r.Heading("role created")
	r.roleResult(role)
}

func (r *Renderer) RoleUpdate(role *management.Role) {
	r.Heading("role updated")
	r.roleResult(role)
}

func (r *Renderer) roleResult(role *management.Role) {
	r.Result(&roleView{
		Name:        role.GetName(),
		ID:          ansi.Faint(role.GetID()),
		Description: role.GetDescription(),
	})
}

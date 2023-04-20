package display

import (
	"fmt"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type roleView struct {
	ID          string
	Name        string
	Description string
	raw         interface{}
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

func (v *roleView) Object() interface{} {
	return v.raw
}

func (r *Renderer) RoleList(roles []*management.Role) {
	resource := "roles"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(roles)))

	if len(roles) == 0 {
		r.EmptyState(resource, "Use 'auth0 roles create' to add one")
		return
	}

	var res []View
	for _, role := range roles {
		res = append(res, makeRoleView(role))
	}

	r.Results(res)
}

func (r *Renderer) UserRoleList(roles []*management.Role) {
	resource := "user roles"
	r.Heading(fmt.Sprintf("%s (%d)", resource, len(roles)))

	if len(roles) == 0 {
		r.EmptyState(resource, "Use 'auth0 users roles assign' to assign roles to a user.")
		return
	}

	var res []View
	for _, role := range roles {
		res = append(res, makeRoleView(role))
	}

	r.Results(res)
}

func (r *Renderer) RoleShow(role *management.Role) {
	r.Heading("role")
	r.Result(makeRoleView(role))
}

func (r *Renderer) RoleCreate(role *management.Role) {
	r.Heading("role created")
	r.Result(makeRoleView(role))
}

func (r *Renderer) RoleUpdate(role *management.Role) {
	r.Heading("role updated")
	r.Result(makeRoleView(role))
}

func makeRoleView(role *management.Role) *roleView {
	return &roleView{
		Name:        role.GetName(),
		ID:          ansi.Faint(role.GetID()),
		Description: role.GetDescription(),
		raw:         role,
	}
}

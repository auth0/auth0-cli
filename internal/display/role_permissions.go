package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type rolePermissionView struct {
	APIID       string
	APIName     string
	Name        string
	Description string
	raw         interface{}
}

func (v *rolePermissionView) Object() interface{} {
	return v.raw
}

func (v *rolePermissionView) AsTableHeader() []string {
	return []string{"API Identifier", "API Name", "Permission Name", "Description"}
}

func (v *rolePermissionView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.APIID),
		v.APIName,
		v.Name,
		v.Description,
	}
}

func (v *rolePermissionView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.APIID)},
		{"NAME", v.APIName},
		{"PERMISSION NAME", v.Name},
		{"DESCRIPTION", v.Description},
	}
}

func (r *Renderer) RolePermissionList(perms []*management.Permission) {
	resource := "role permissions"

	r.Heading(resource)

	if len(perms) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 roles permissions associate' to add one")
		return
	}

	var res []View
	for _, perm := range perms {
		res = append(res, &rolePermissionView{
			APIName:     perm.GetResourceServerName(),
			APIID:       perm.GetResourceServerIdentifier(),
			Name:        perm.GetName(),
			Description: perm.GetDescription(),
			raw:         perm,
		})
	}

	r.Results(res)
}

/*
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
*/

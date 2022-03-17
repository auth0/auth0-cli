package display

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/go-auth0/management"
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
		r.Infof("Use 'auth0 roles permissions add' to add one")
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

func (r *Renderer) RolePermissionAdd(role *management.Role, rs *management.ResourceServer, perms []string) {
	r.Heading("role permissions added")

	r.Infof("Added permissions %s (%s) to role %s.", ansi.Green(strings.Join(perms, ", ")), ansi.Faint(rs.GetIdentifier()), ansi.Green(role.GetName()))
}

func (r *Renderer) RolePermissionRemove(role *management.Role, rs *management.ResourceServer, perms []string) {
	r.Heading("role permissions removed")

	r.Infof("Removed permissions %s (%s) from role %s.", ansi.Green(strings.Join(perms, ", ")), ansi.Faint(rs.GetIdentifier()), ansi.Green(role.GetName()))
}

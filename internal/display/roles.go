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

type permissionView struct {
	Name                     string
	ResourceServerIdentifier string
	ResourceServerName       string
	Description              string
}

func (v *roleView) AsTableHeader() []string {
	return []string{"Name", "Role ID", "Description"}
}

func (v *roleView) AsTableRow() []string {
	return []string{v.Name, v.ID, v.Description}
}

func (v *permissionView) AsTableHeader() []string {
	return []string{"Permission Name", "Resource Service Identifier", "Resource Server Name", "Description"}
}

func (v *permissionView) AsTableRow() []string {
	return []string{v.Name, v.ResourceServerIdentifier, v.ResourceServerName, v.Description}
}

func (r *Renderer) RoleList(roles []*management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "roles\n")
	var res []View
	for _, r := range roles {
		res = append(res, &roleView{
			Name:        auth0.StringValue(r.Name),
			ID:          auth0.StringValue(r.ID),
			Description: auth0.StringValue(r.Description),
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

func (r *Renderer) RoleGetPermissions(permissions []*management.Permission) {
	r.Heading(ansi.Bold(r.Tenant), "permissions\n")
	var res []View
	for _, p := range permissions {
		res = append(res, &permissionView{
			ResourceServerIdentifier: auth0.StringValue(p.ResourceServerIdentifier),
			ResourceServerName:       auth0.StringValue(p.ResourceServerName),
			Name:                     auth0.StringValue(p.Name),
			Description:              auth0.StringValue(p.Description),
		})
	}

	r.Results(res)
}

package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type roleView struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
}

type roleSingleView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type permissionView struct {
	RoleID                   string `json:"id"`
	Name                     string `json:"name"`
	ResourceServerIdentifier string `json:"resource_server_identifier,omitempty"`
	ResourceServerName       string `json:"resource_server_name,omitempty"`
	Description              string `json:"description,omitempty"`
}

func (v *roleView) AsTableHeader() []string {
	return []string{"Role ID", "Name", "Description"}
}

func (v *roleView) AsTableRow() []string {
	return []string{v.ID, v.Name, v.Description}
}

func (v *roleSingleView) AsTableHeader() []string {
	return []string{}
}

func (v *roleSingleView) AsTableRow() []string {
	return []string{v.Name, v.Value}
}

func (v *permissionView) AsTableHeader() []string {
	return []string{"Role ID", "Permission Name", "Description", "Resource Service Identifier", "Resource Server Name"}
}

func (v *permissionView) AsTableRow() []string {
	return []string{v.RoleID, v.Name, v.Description, v.ResourceServerIdentifier, v.ResourceServerName}
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
	views := []View{
		&roleSingleView{Name: "ROLE ID", Value: auth0.StringValue(role.ID)},
		&roleSingleView{Name: "NAME", Value: auth0.StringValue(role.Name)},
	}
	if auth0.StringValue(role.Description) != "" {
		views = append(views, &roleSingleView{Name: "DESCRIPTION", Value: auth0.StringValue(role.Description)})
	}
	r.Results(views)
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

func (r *Renderer) RoleGetPermissions(roleID string, permissions []*management.Permission) {
	r.Heading(ansi.Bold(r.Tenant), "role permissions\n")
	var res []View
	for _, p := range permissions {
		res = append(res, &permissionView{
			RoleID:                   roleID,
			ResourceServerIdentifier: auth0.StringValue(p.ResourceServerIdentifier),
			ResourceServerName:       auth0.StringValue(p.ResourceServerName),
			Name:                     auth0.StringValue(p.Name),
			Description:              auth0.StringValue(p.Description),
		})
	}

	r.Results(res)
}

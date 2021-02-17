package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type roleView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type roleSingleView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type rolePermissionView struct {
	ID                       string `json:"id"`
	PermissionName           string `json:"permission_name"`
	ResourceServerIdentifier string `json:"resource_server_identifier,omitempty"`
}

type roleUserView struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	UserEmail string `json:"email,omitempty"`
}

type roleUserSingleView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type rolePermissionSingleView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

func (v *rolePermissionView) AsTableHeader() []string {
	return []string{"Role ID", "Permission Name", "Resource Server Identifier"}
}

func (v *rolePermissionView) AsTableRow() []string {
	return []string{v.ID, v.PermissionName, v.ResourceServerIdentifier}
}

func (v *rolePermissionSingleView) AsTableHeader() []string {
	return []string{}
}

func (v *rolePermissionSingleView) AsTableRow() []string {
	return []string{v.Name, v.Value}
}

func (v *roleUserView) AsTableHeader() []string {
	return []string{"Role ID", "User ID", "User Email"}
}

func (v *roleUserView) AsTableRow() []string {
	return []string{v.ID, v.UserID, v.UserEmail}
}

func (v *roleUserSingleView) AsTableHeader() []string {
	return []string{}
}

func (v *roleUserSingleView) AsTableRow() []string {
	return []string{v.Name, v.Value}
}

func (r *Renderer) RoleList(roles []*management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "roles\n")
	var v []View
	for _, r := range roles {
		v = append(v, &roleView{
			Name:        auth0.StringValue(r.Name),
			ID:          auth0.StringValue(r.ID),
			Description: auth0.StringValue(r.Description),
		})
	}
	r.Results(v)
}

func (r *Renderer) RoleGet(role *management.Role) {
	r.Heading(ansi.Bold(r.Tenant), "role\n")
	v := []View{
		&roleSingleView{Name: "ROLE ID", Value: auth0.StringValue(role.ID)},
		&roleSingleView{Name: "NAME", Value: auth0.StringValue(role.Name)},
	}
	if auth0.StringValue(role.Description) != "" {
		v = append(v, &roleSingleView{Name: "DESCRIPTION", Value: auth0.StringValue(role.Description)})
	}
	r.Results(v)
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

func (r *Renderer) RolePermissionsList(rolesPermissions map[string][]*management.Permission) {
	r.Heading(ansi.Bold(r.Tenant), "role permissions\n")
	var v []View
	for roleID, permissions := range rolesPermissions {
		for _, permission := range permissions {
			v = append(v, &rolePermissionView{
				ID:                       roleID,
				PermissionName:           auth0.StringValue(permission.Name),
				ResourceServerIdentifier: auth0.StringValue(permission.ResourceServerIdentifier),
			})
		}
	}
	r.Results(v)
}

func (r *Renderer) RolePermissionsGet(roleID string, permissions []*management.Permission) {
	r.Heading(ansi.Bold(r.Tenant), "role permissions\n")
	v := []View{
		&rolePermissionSingleView{Name: "ROLE ID", Value: roleID},
	}
	for _, p := range permissions {
		v = append(v,
			&rolePermissionSingleView{Name: "PERMISSION NAME", Value: auth0.StringValue(p.Name)},
			&rolePermissionSingleView{Name: "RESOURCE SERVER IDENTIFIER", Value: auth0.StringValue(p.ResourceServerIdentifier)},
		)
	}
	r.Results(v)
}

func (r *Renderer) RoleUsersList(rolesUsers map[string][]*management.User) {
	r.Heading(ansi.Bold(r.Tenant), "role users\n")
	var v []View
	for roleID, users := range rolesUsers {
		for _, user := range users {
			v = append(v, &roleUserView{
				ID:        roleID,
				UserID:    auth0.StringValue(user.ID),
				UserEmail: auth0.StringValue(user.Email),
			})
		}
	}
	r.Results(v)
}

func (r *Renderer) RoleUsersGet(roleID string, users []*management.User) {
	r.Heading(ansi.Bold(r.Tenant), "role users\n")
	v := []View{
		&rolePermissionSingleView{Name: "ROLE ID", Value: roleID},
	}
	for _, u := range users {
		v = append(v,
			&roleUserSingleView{Name: "USER ID", Value: auth0.StringValue(u.ID)},
			&roleUserSingleView{Name: "USER EMAIL", Value: auth0.StringValue(u.Email)},
		)
	}
	r.Results(v)
}

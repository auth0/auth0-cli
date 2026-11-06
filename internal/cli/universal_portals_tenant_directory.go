package cli

import (
	"context"
	"strings"

	goauth0 "github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/portals"
)

// ReadDefaultCustomDomain implements portals.CustomDomainStore.
func (t *portalsTenant) ReadDefaultCustomDomain(ctx context.Context) (portals.CustomDomain, error) {
	domain, err := t.api.CustomDomain.ReadDefault(ctx)
	if err != nil {
		return portals.CustomDomain{}, asPortalsError(err)
	}

	return portals.CustomDomain{Domain: domain.GetDomain(), Status: domain.GetStatus()}, nil
}

// SearchUsersByEmail implements portals.UserStore.
func (t *portalsTenant) SearchUsersByEmail(ctx context.Context, email string) ([]portals.User, error) {
	users, err := listPages(
		ctx,
		func(opts ...management.RequestOption) ([]*management.User, bool, error) {
			list, err := t.api.User.Search(ctx, opts...)
			if err != nil {
				return nil, false, err
			}

			return list.Users, list.HasNext(), nil
		},
		management.Query(`email:"`+escapeSearchPhrase(email)+`"`),
	)
	if err != nil {
		return nil, err
	}

	return usersFromSDK(users), nil
}

// escapeSearchPhrase escapes the characters that end or change a quoted phrase of the user search.
func escapeSearchPhrase(phrase string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(phrase)
}

// ReadUser implements portals.UserStore.
func (t *portalsTenant) ReadUser(ctx context.Context, id string) (portals.User, error) {
	user, err := t.api.User.Read(ctx, id)
	if err != nil {
		return portals.User{}, asPortalsError(err)
	}

	return userFromSDK(user), nil
}

func usersFromSDK(users []*management.User) []portals.User {
	listed := make([]portals.User, 0, len(users))
	for _, user := range users {
		listed = append(listed, userFromSDK(user))
	}

	return listed
}

func userFromSDK(user *management.User) portals.User {
	return portals.User{ID: user.GetID(), Email: user.GetEmail(), Name: user.GetName()}
}

// ListUserOrganizations implements portals.UserStore.
func (t *portalsTenant) ListUserOrganizations(ctx context.Context, userID string) ([]portals.Organization, error) {
	organizations, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.Organization, bool, error) {
		list, err := t.api.User.Organizations(ctx, userID, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.Organizations, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	return organizationsFromSDK(organizations), nil
}

// ListOrganizations implements portals.OrganizationStore.
func (t *portalsTenant) ListOrganizations(ctx context.Context) ([]portals.Organization, error) {
	organizations, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.Organization, bool, error) {
		list, err := t.api.Organization.List(ctx, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.Organizations, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	return organizationsFromSDK(organizations), nil
}

// ReadOrganization implements portals.OrganizationStore.
func (t *portalsTenant) ReadOrganization(ctx context.Context, id string) (portals.Organization, error) {
	organization, err := t.api.Organization.Read(ctx, id)
	if err != nil {
		return portals.Organization{}, asPortalsError(err)
	}

	return organizationFromSDK(organization), nil
}

// CreateOrganization implements portals.OrganizationStore.
func (t *portalsTenant) CreateOrganization(ctx context.Context, spec portals.OrganizationSpec) (portals.Organization, error) {
	organization := &management.Organization{
		Name:        goauth0.String(spec.Name),
		DisplayName: goauth0.String(spec.DisplayName),
	}

	if err := t.api.Organization.Create(ctx, organization); err != nil {
		return portals.Organization{}, asPortalsError(err)
	}

	return organizationFromSDK(organization), nil
}

func organizationsFromSDK(organizations []*management.Organization) []portals.Organization {
	listed := make([]portals.Organization, 0, len(organizations))
	for _, organization := range organizations {
		listed = append(listed, organizationFromSDK(organization))
	}

	return listed
}

func organizationFromSDK(organization *management.Organization) portals.Organization {
	return portals.Organization{
		ID:          organization.GetID(),
		Name:        organization.GetName(),
		DisplayName: organization.GetDisplayName(),
	}
}

// ListOrganizationConnections implements portals.OrganizationStore.
func (t *portalsTenant) ListOrganizationConnections(ctx context.Context, id string) ([]string, error) {
	connections, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.OrganizationConnection, bool, error) {
		list, err := t.api.Organization.Connections(ctx, id, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.OrganizationConnections, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(connections))
	for _, connection := range connections {
		ids = append(ids, connection.GetConnectionID())
	}

	return ids, nil
}

// EnableOrganizationConnection implements portals.OrganizationStore.
func (t *portalsTenant) EnableOrganizationConnection(ctx context.Context, id, connectionID string) error {
	return asPortalsError(t.api.Organization.AddConnection(ctx, id, &management.OrganizationConnection{
		ConnectionID:            goauth0.String(connectionID),
		AssignMembershipOnLogin: goauth0.Bool(false),
	}))
}

// AddOrganizationMember implements portals.OrganizationStore.
func (t *portalsTenant) AddOrganizationMember(ctx context.Context, id, userID string) error {
	return asPortalsError(t.api.Organization.AddMembers(ctx, id, []string{userID}))
}

// ListMemberRoles implements portals.OrganizationStore.
func (t *portalsTenant) ListMemberRoles(ctx context.Context, id, userID string) ([]portals.Role, error) {
	roles, err := listPages(ctx, func(opts ...management.RequestOption) ([]management.OrganizationMemberRole, bool, error) {
		list, err := t.api.Organization.MemberRoles(ctx, id, userID, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.Roles, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.Role, 0, len(roles))
	for _, role := range roles {
		listed = append(listed, portals.Role{ID: role.GetID(), Name: role.GetName()})
	}

	return listed, nil
}

// AssignMemberRole implements portals.OrganizationStore.
func (t *portalsTenant) AssignMemberRole(ctx context.Context, id, userID, roleID string) error {
	return asPortalsError(t.api.Organization.AssignMemberRoles(ctx, id, userID, []string{roleID}))
}

// ListRoles implements portals.RoleStore.
func (t *portalsTenant) ListRoles(ctx context.Context) ([]portals.Role, error) {
	roles, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.Role, bool, error) {
		list, err := t.api.Role.List(ctx, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.Roles, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.Role, 0, len(roles))
	for _, role := range roles {
		listed = append(listed, portals.Role{ID: role.GetID(), Name: role.GetName()})
	}

	return listed, nil
}

// CreateRole implements portals.RoleStore.
func (t *portalsTenant) CreateRole(ctx context.Context, spec portals.RoleSpec) (portals.Role, error) {
	role := &management.Role{
		Name:        goauth0.String(spec.Name),
		Description: goauth0.String(spec.Description),
	}

	if err := t.api.Role.Create(ctx, role); err != nil {
		return portals.Role{}, asPortalsError(err)
	}

	return portals.Role{ID: role.GetID(), Name: role.GetName()}, nil
}

// ListRolePermissions implements portals.RoleStore.
func (t *portalsTenant) ListRolePermissions(ctx context.Context, roleID string) ([]portals.Permission, error) {
	permissions, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.Permission, bool, error) {
		list, err := t.api.Role.Permissions(ctx, roleID, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.Permissions, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.Permission, 0, len(permissions))
	for _, permission := range permissions {
		listed = append(listed, portals.Permission{
			Name:                     permission.GetName(),
			ResourceServerIdentifier: permission.GetResourceServerIdentifier(),
		})
	}

	return listed, nil
}

// AddRolePermissions implements portals.RoleStore.
func (t *portalsTenant) AddRolePermissions(ctx context.Context, roleID string, permissions []portals.Permission) error {
	sdkPermissions := make([]*management.Permission, 0, len(permissions))
	for _, permission := range permissions {
		sdkPermissions = append(sdkPermissions, &management.Permission{
			Name:                     goauth0.String(permission.Name),
			ResourceServerIdentifier: goauth0.String(permission.ResourceServerIdentifier),
		})
	}

	return asPortalsError(t.api.Role.AssociatePermissions(ctx, roleID, sdkPermissions))
}

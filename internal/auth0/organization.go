//go:generate mockgen -source=organization.go -destination=mock/organization_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type OrganizationAPI interface {
	// Create an Organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_organizations
	Create(ctx context.Context, o *management.Organization, opts ...management.RequestOption) error

	// Read a specific organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations_by_id
	Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.Organization, error)

	// Update an organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/patch_organizations_by_id
	Update(ctx context.Context, id string, o *management.Organization, opts ...management.RequestOption) error

	// Delete a specific organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_organizations_by_id
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// List available organizations.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations
	List(ctx context.Context, opts ...management.RequestOption) (c *management.OrganizationList, err error)

	// Members lists members of an organization.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Organizations/get_members
	Members(ctx context.Context, id string, opts ...management.RequestOption) (o *management.OrganizationMemberList, err error)

	// MemberRoles lists roles assigned to a member of an organization
	//
	// See: https://auth0.com/docs/api/management/v2#!/Organizations/get_organization_member_roles
	MemberRoles(ctx context.Context, id string, userID string, opts ...management.RequestOption) (r *management.OrganizationMemberRoleList, err error)

	// Connections retrieves connections enabled for an organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_enabled_connections
	Connections(ctx context.Context, id string, opts ...management.RequestOption) (c *management.OrganizationConnectionList, err error)
}

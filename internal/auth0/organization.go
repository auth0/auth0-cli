package auth0

import "github.com/auth0/go-auth0/management"

type OrganizationAPI interface {
	// Create an Organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_organizations
	Create(o *management.Organization, opts ...management.RequestOption) error

	// Read a specific organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations_by_id
	Read(id string, opts ...management.RequestOption) (*management.Organization, error)

	// Update an organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/patch_organizations_by_id
	Update(id string, o *management.Organization, opts ...management.RequestOption) error

	// Delete a specific organization.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_organizations_by_id
	Delete(id string, opts ...management.RequestOption) error

	// List available organizations.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations
	List(opts ...management.RequestOption) (c *management.OrganizationList, err error)

	// Members lists members of an organization.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Organizations/get_members
	Members(id string, opts ...management.RequestOption) (o *management.OrganizationMemberList, err error)

	// MemberRoles lists roles assigned to a member of an organization
	//
	// See: https://auth0.com/docs/api/management/v2#!/Organizations/get_organization_member_roles
	MemberRoles(id string, userID string, opts ...management.RequestOption) (r *management.OrganizationMemberRoleList, err error)
}

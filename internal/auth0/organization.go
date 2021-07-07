package auth0

import "gopkg.in/auth0.v5/management"

type OrganizationAPI interface {
	// Create an Organization
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_organizations
	Create(o *management.Organization, opts ...management.RequestOption) error

	// Get a specific organization
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations_by_id
	Read(id string, opts ...management.RequestOption) (*management.Organization, error)

	// Modify an organization
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/patch_organizations_by_id
	Update(o *management.Organization, opts ...management.RequestOption) error

	// Delete a specific organization
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_organizations_by_id
	Delete(id string, opts ...management.RequestOption) error

	// List available organizations
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations
	List(opts ...management.RequestOption) (c *management.OrganizationList, err error)
}

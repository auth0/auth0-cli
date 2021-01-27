//go:generate mockgen -source=custom_domain.go -destination=custom_domain_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type CustomDomainAPI interface {
	// Create a new custom domain.
	Create(r *management.CustomDomain, opts ...management.RequestOption) (err error)

	// Retrieve a custom domain configuration and status.
	Read(id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// Run the verification process on a custom domain.
	Verify(id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// Delete a custom domain and stop serving requests for it.
	Delete(id string, opts ...management.RequestOption) (err error)

	// List all custom domains.
	List(opts ...management.RequestOption) (c []*management.CustomDomain, err error)
}

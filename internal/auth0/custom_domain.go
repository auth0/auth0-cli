//go:generate mockgen -source=custom_domain.go -destination=custom_domain_mock.go -package=auth0
package auth0

import "gopkg.in/auth0.v5/management"

type CustomDomainAPI interface {
	// Create a new custom domain.
	Create(c *management.CustomDomain, opts ...management.RequestOption) (err error)

	// Read retrieves a custom domain by its id.
	Read(id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// Delete a custom domain.
	Delete(id string, opts ...management.RequestOption) (err error)

	// Verify a custom domain.
	Verify(id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// List all custom domains.
	List(opts ...management.RequestOption) (c []*management.CustomDomain, err error)
}

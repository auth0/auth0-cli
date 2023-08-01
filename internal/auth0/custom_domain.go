//go:generate mockgen -source=custom_domain.go -destination=mock/custom_domain_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type CustomDomainAPI interface {
	// Create a new custom domain.
	Create(ctx context.Context, c *management.CustomDomain, opts ...management.RequestOption) (err error)

	// Read retrieves a custom domain by its id.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// Update a custom domain.
	Update(ctx context.Context, id string, c *management.CustomDomain, opts ...management.RequestOption) (err error)

	// Delete a custom domain.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)

	// Verify a custom domain.
	Verify(ctx context.Context, id string, opts ...management.RequestOption) (c *management.CustomDomain, err error)

	// List all custom domains.
	List(ctx context.Context, opts ...management.RequestOption) (c []*management.CustomDomain, err error)
}

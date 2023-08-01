//go:generate mockgen -source=resource_server.go -destination=mock/resource_server_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ResourceServerAPI interface {
	// Create a resource server.
	Create(ctx context.Context, rs *management.ResourceServer, opts ...management.RequestOption) (err error)

	// Read retrieves a resource server by its id or audience.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (rs *management.ResourceServer, err error)

	// Update a resource server.
	Update(ctx context.Context, id string, rs *management.ResourceServer, opts ...management.RequestOption) (err error)

	// Delete a resource server.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)

	// List all resource server.
	List(ctx context.Context, opts ...management.RequestOption) (rl *management.ResourceServerList, err error)
}

//go:generate mockgen -source=client.go -destination=mock/client_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ClientAPI interface {
	// Create a new client application.
	Create(ctx context.Context, c *management.Client, opts ...management.RequestOption) (err error)

	// Read a client by its id.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (c *management.Client, err error)

	// List all client applications.
	List(ctx context.Context, opts ...management.RequestOption) (c *management.ClientList, err error)

	// Update a client.
	Update(ctx context.Context, id string, c *management.Client, opts ...management.RequestOption) (err error)

	// RotateSecret rotates a client secret.
	RotateSecret(ctx context.Context, id string, opts ...management.RequestOption) (c *management.Client, err error)

	// Delete a client and all its related assets (like rules, connections, etc)
	// given its id.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error
}

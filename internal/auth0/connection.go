//go:generate mockgen -source=connection.go -destination=mock/connection_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ConnectionAPI interface {

	// Create a new connection.
	Create(ctx context.Context, c *management.Connection, opts ...management.RequestOption) (err error)

	// Read retrieves a connection by its id.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (c *management.Connection, err error)

	// ReadByName retrieves a connection by its name.
	ReadByName(ctx context.Context, id string, opts ...management.RequestOption) (c *management.Connection, err error)

	// Update a connection.
	Update(ctx context.Context, id string, c *management.Connection, opts ...management.RequestOption) (err error)

	// Delete a connection.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)

	// List all connections.
	List(ctx context.Context, opts ...management.RequestOption) (ul *management.ConnectionList, err error)
}

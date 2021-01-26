//go:generate mockgen -source=connection.go -destination=connection_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type ConnectionAPI interface {
	// Create a new connection.
	Create(c *management.Connection, opts ...management.RequestOption) error

	// Read retrieves a connection by its id.
	Read(id string, opts ...management.RequestOption) (c *management.Connection, err error)

	// List all connections.
	List(opts ...management.RequestOption) (c *management.ConnectionList, err error)

	// Update a connection.
	Update(id string, c *management.Connection, opts ...management.RequestOption) (err error)

	// Delete a connection and all its users.
	Delete(id string, opts ...management.RequestOption) (err error)

	// ReadByName retrieves a connection by its name. This is a helper method when a
	// connection id is not readily available.
	ReadByName(name string, opts ...management.RequestOption) (*management.Connection, error)
}

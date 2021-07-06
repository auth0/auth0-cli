package auth0

import "gopkg.in/auth0.v5/management"

type ConnectionAPI interface {

	// Create a new connection.
	Create(c *management.Connection, opts ...management.RequestOption) (err error)

	// Read retrieves a connection by its id.
	Read(id string, opts ...management.RequestOption) (c *management.Connection, err error)

	// ReadByName retrieves a connection by its name.
	ReadByName(id string, opts ...management.RequestOption) (c *management.Connection, err error)

	// Update a connection.
	Update(id string, c *management.Connection, opts ...management.RequestOption) (err error)

	// Delete a connection.
	Delete(id string, opts ...management.RequestOption) (err error)

	// List all connections.
	List(opts ...management.RequestOption) (ul *management.ConnectionList, err error)
}

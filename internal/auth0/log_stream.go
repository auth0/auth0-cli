//go:generate mockgen -source=log_stream.go -destination=log_stream_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type LogStreamAPI interface {
	// Create a log stream.
	Create(ls *management.LogStream, opts ...management.RequestOption) (err error)

	// Read a log stream.
	Read(id string, opts ...management.RequestOption) (ls *management.LogStream, err error)

	// Update a log stream.
	Update(id string, ls *management.LogStream, opts ...management.RequestOption) (err error)

	// List all log streams.
	List(opts ...management.RequestOption) (ls []*management.LogStream, err error)

	// Delete a log stream.
	Delete(id string, opts ...management.RequestOption) (err error)
}

//go:generate mockgen -source=log_stream.go -destination=mock/log_stream_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type LogStreamAPI interface {
	// Create a log stream.
	Create(ctx context.Context, ls *management.LogStream, opts ...management.RequestOption) (err error)

	// Read a log stream.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (ls *management.LogStream, err error)

	// Update a log stream.
	Update(ctx context.Context, id string, ls *management.LogStream, opts ...management.RequestOption) (err error)

	// List all log streams.
	List(ctx context.Context, opts ...management.RequestOption) (ls []*management.LogStream, err error)

	// Delete a log stream.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)
}

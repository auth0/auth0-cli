//go:generate mockgen -source=event_streams.go -destination=mock/event_streams_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type EventStreamAPI interface {
	// Create a new Event Stream.
	Create(ctx context.Context, e *management.EventStream, opts ...management.RequestOption) error

	// Read Event Stream details.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (e *management.EventStream, err error)

	// Update an existing Event Stream.
	Update(ctx context.Context, id string, e *management.EventStream, opts ...management.RequestOption) error

	// Delete an Event Stream.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// List Event Streams.
	List(ctx context.Context, opts ...management.RequestOption) (e *management.EventStreamList, err error)
}

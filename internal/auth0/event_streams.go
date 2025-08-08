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

	// Test triggers a test event on an Event Stream.
	Test(ctx context.Context, id string, testEvent *management.TestEvent, opts ...management.RequestOption) error

	// ReadDelivery returns delivery information for a specific event associated to an Event Stream.
	ReadDelivery(ctx context.Context, streamID, deliveryID string, opts ...management.RequestOption) (ed *management.EventDelivery, err error)

	// ListDeliveries returns delivery attempts for all events associated to an Event Stream.
	ListDeliveries(ctx context.Context, id string, opts ...management.RequestOption) (edl *management.EventDeliveryList, err error)

	// Stats returns event stream statistics.
	Stats(ctx context.Context, id string, opts ...management.RequestOption) (stats *management.EventStreamStats, err error)

	// Redeliver a single failed delivery by ID.
	Redeliver(ctx context.Context, streamID, deliveryID string, opts ...management.RequestOption) error

	// RedeliverMany retries multiple failed deliveries.
	RedeliverMany(ctx context.Context, streamID string, req *management.BulkRedeliverRequest, opts ...management.RequestOption) error
}

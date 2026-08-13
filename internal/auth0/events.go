package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// EventsAPIV3 is the V3 SDK interface for the /events endpoint
// (Server-Sent Event subscription stream).
type EventsAPIV3 interface {
	// Subscribe to events via Server-Sent Events (SSE).
	//
	// Required scope: `read:events`
	//
	// See: https://auth0.com/docs/api/management/v2/events/get-events
	Subscribe(
		ctx context.Context,
		request *managementv3.SubscribeEventsRequestParameters,
		opts ...option.RequestOption,
	) (*core.Stream[managementv3.EventStreamSubscribeEventsResponseContent], error)
}

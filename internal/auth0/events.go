package auth0

import (
	"context"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/auth0/go-auth0/v2/management/core"
	"github.com/auth0/go-auth0/v2/management/option"
)

// EventsAPIV2 is the V2 SDK interface for the /events endpoint
// (Server-Sent Event subscription stream).
type EventsAPIV2 interface {
	// Subscribe to events via Server-Sent Events (SSE).
	//
	// Required scope: `read:events`
	//
	// See: https://auth0.com/docs/api/management/v2/events/get-events
	Subscribe(
		ctx context.Context,
		request *managementv2.SubscribeEventsRequestParameters,
		opts ...option.RequestOption,
	) (*core.Stream[managementv2.EventStreamSubscribeEventsResponseContent], error)
}

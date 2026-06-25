//go:generate mockgen -source=segments.go -destination=mock/segments_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	managementoption "github.com/auth0/go-auth0/v2/management/option"
)

// SegmentsAPI describes the interface for segment operations.
type SegmentsAPI interface {
	List(ctx context.Context, request *management.ListSegmentsRequestParameters, opts ...managementoption.RequestOption) (*managementcore.Page[*string, *management.Segment, *management.ListSegmentsResponseContent], error)
	Create(ctx context.Context, request *management.CreateSegmentRequestContent, opts ...managementoption.RequestOption) (*management.CreateSegmentResponseContent, error)
	Get(ctx context.Context, id string, opts ...managementoption.RequestOption) (*management.GetSegmentResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateSegmentRequestContent, opts ...managementoption.RequestOption) (*management.UpdateSegmentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...managementoption.RequestOption) error
}

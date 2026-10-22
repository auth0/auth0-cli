//go:generate go tool mockgen -source=segments.go -destination=mock/segments_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// SegmentListPage aliases the checkpoint-paginated segments list response. The
// alias keeps the interface return type a single identifier so mockgen's source
// parser can handle it (it cannot parse the multi-type-parameter generic inline).
type SegmentListPage = core.Page[*string, *management.Segment, *management.ListSegmentsResponseContent]

// SegmentsAPI describes the interface for segment operations.
type SegmentsAPI interface {
	List(ctx context.Context, request *management.ListSegmentsRequestParameters, opts ...option.RequestOption) (*SegmentListPage, error)
	Create(ctx context.Context, request *management.CreateSegmentRequestContent, opts ...option.RequestOption) (*management.CreateSegmentResponseContent, error)
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*management.GetSegmentResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateSegmentRequestContent, opts ...option.RequestOption) (*management.UpdateSegmentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

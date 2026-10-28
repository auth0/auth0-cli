//go:generate go tool mockgen -source=feature_flags.go -destination=mock/feature_flags_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// FeatureFlagListPage aliases the checkpoint-paginated feature flags list
// response. The alias keeps the interface return type a single identifier so
// mockgen's source parser can handle it (it cannot parse the multi-type-parameter
// generic inline).
type FeatureFlagListPage = core.Page[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent]

// FeatureFlagsAPI describes the interface for feature flag operations.
type FeatureFlagsAPI interface {
	List(ctx context.Context, request *management.ListFeatureFlagsRequestParameters, opts ...option.RequestOption) (*FeatureFlagListPage, error)
	Create(ctx context.Context, request *management.CreateFeatureFlagRequestContent, opts ...option.RequestOption) (*management.CreateFeatureFlagResponseContent, error)
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*management.GetFeatureFlagResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateFeatureFlagRequestContent, opts ...option.RequestOption) (*management.UpdateFeatureFlagResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

// VariationsAPI describes the interface for variation operations (nested under feature flags).
type VariationsAPI interface {
	List(ctx context.Context, featureFlagID string, opts ...option.RequestOption) (*management.ListVariationsResponseContent, error)
	Create(ctx context.Context, featureFlagID string, request *management.CreateVariationRequestContent, opts ...option.RequestOption) (*management.CreateVariationResponseContent, error)
	Get(ctx context.Context, featureFlagID string, variationID string, opts ...option.RequestOption) (*management.GetVariationResponseContent, error)
	Update(ctx context.Context, featureFlagID string, variationID string, request *management.UpdateVariationRequestContent, opts ...option.RequestOption) (*management.UpdateVariationResponseContent, error)
	Delete(ctx context.Context, featureFlagID string, variationID string, opts ...option.RequestOption) error
}

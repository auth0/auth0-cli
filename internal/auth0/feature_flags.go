//go:generate mockgen -source=feature_flags.go -destination=mock/feature_flags_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	managementoption "github.com/auth0/go-auth0/v2/management/option"
)

// FeatureFlagsAPI describes the interface for feature flag operations.
type FeatureFlagsAPI interface {
	List(ctx context.Context, request *management.ListFeatureFlagsRequestParameters, opts ...managementoption.RequestOption) (*managementcore.Page[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent], error)
	Create(ctx context.Context, request *management.CreateFeatureFlagRequestContent, opts ...managementoption.RequestOption) (*management.CreateFeatureFlagResponseContent, error)
	Get(ctx context.Context, id string, opts ...managementoption.RequestOption) (*management.GetFeatureFlagResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateFeatureFlagRequestContent, opts ...managementoption.RequestOption) (*management.UpdateFeatureFlagResponseContent, error)
	Delete(ctx context.Context, id string, opts ...managementoption.RequestOption) error
	UpdateStatus(ctx context.Context, id string, request *management.UpdateFeatureFlagStatusRequestContent, opts ...managementoption.RequestOption) (*management.UpdateFeatureFlagStatusResponseContent, error)
}

// VariationsAPI describes the interface for variation operations (nested under feature flags).
type VariationsAPI interface {
	List(ctx context.Context, featureFlagID string, opts ...managementoption.RequestOption) (*management.ListVariationsResponseContent, error)
	Create(ctx context.Context, featureFlagID string, request *management.CreateVariationRequestContent, opts ...managementoption.RequestOption) (*management.CreateVariationResponseContent, error)
	Get(ctx context.Context, featureFlagID string, variationID string, opts ...managementoption.RequestOption) (*management.GetVariationResponseContent, error)
	Update(ctx context.Context, featureFlagID string, variationID string, request *management.UpdateVariationRequestContent, opts ...managementoption.RequestOption) (*management.UpdateVariationResponseContent, error)
	Delete(ctx context.Context, featureFlagID string, variationID string, opts ...managementoption.RequestOption) error
}

//go:generate mockgen -source=guardian_factor_push.go -destination=mock/guardian_factor_push_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianFactorPushAPIV3 is the V3 SDK interface for the push-notification MFA
// factor configuration (/guardian/factors/push-notification), including its
// APNs, FCM, FCM v1 and SNS providers.
//
// Required scopes: `read:guardian_factors` for reads, `update:guardian_factors`
// for writes.
type GuardianFactorPushAPIV3 interface {
	// GetSelectedProvider retrieves the configured push-notification provider.
	GetSelectedProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderPushNotificationResponseContent, error)

	// SetProvider sets the push-notification provider.
	SetProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPushNotificationRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderPushNotificationResponseContent, error)

	// GetApnsProvider retrieves the Apple APNs configuration.
	GetApnsProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderApnsResponseContent, error)

	// SetApnsProvider replaces the Apple APNs configuration.
	SetApnsProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPushNotificationApnsRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderPushNotificationApnsResponseContent, error)

	// UpdateApnsProvider partially updates the Apple APNs configuration.
	UpdateApnsProvider(ctx context.Context, request *managementv3.UpdateGuardianFactorsProviderPushNotificationApnsRequestContent, opts ...option.RequestOption) (*managementv3.UpdateGuardianFactorsProviderPushNotificationApnsResponseContent, error)

	// SetFcmProvider replaces the Google FCM (legacy) configuration.
	SetFcmProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPushNotificationFcmRequestContent, opts ...option.RequestOption) (managementv3.SetGuardianFactorsProviderPushNotificationFcmResponseContent, error)

	// UpdateFcmProvider partially updates the Google FCM (legacy) configuration.
	UpdateFcmProvider(ctx context.Context, request *managementv3.UpdateGuardianFactorsProviderPushNotificationFcmRequestContent, opts ...option.RequestOption) (managementv3.UpdateGuardianFactorsProviderPushNotificationFcmResponseContent, error)

	// SetFcmv1Provider replaces the Google FCM v1 configuration.
	SetFcmv1Provider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPushNotificationFcmv1RequestContent, opts ...option.RequestOption) (managementv3.SetGuardianFactorsProviderPushNotificationFcmv1ResponseContent, error)

	// UpdateFcmv1Provider partially updates the Google FCM v1 configuration.
	UpdateFcmv1Provider(ctx context.Context, request *managementv3.UpdateGuardianFactorsProviderPushNotificationFcmv1RequestContent, opts ...option.RequestOption) (managementv3.UpdateGuardianFactorsProviderPushNotificationFcmv1ResponseContent, error)

	// GetSnsProvider retrieves the Amazon SNS configuration.
	GetSnsProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderSnsResponseContent, error)

	// SetSnsProvider replaces the Amazon SNS configuration.
	SetSnsProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPushNotificationSnsRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderPushNotificationSnsResponseContent, error)

	// UpdateSnsProvider partially updates the Amazon SNS configuration.
	UpdateSnsProvider(ctx context.Context, request *managementv3.UpdateGuardianFactorsProviderPushNotificationSnsRequestContent, opts ...option.RequestOption) (*managementv3.UpdateGuardianFactorsProviderPushNotificationSnsResponseContent, error)
}

//go:generate mockgen -source=guardian_factor_sms.go -destination=mock/guardian_factor_sms_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianFactorSmsAPIV3 is the V3 SDK interface for the SMS MFA factor
// configuration (/guardian/factors/sms).
//
// Required scopes: `read:guardian_factors` for reads, `update:guardian_factors`
// for writes.
type GuardianFactorSmsAPIV3 interface {
	// GetSelectedProvider retrieves the configured SMS provider.
	GetSelectedProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderSmsResponseContent, error)

	// SetProvider sets the SMS provider.
	SetProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderSmsRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderSmsResponseContent, error)

	// GetTemplates retrieves the SMS enrollment and verification templates.
	GetTemplates(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorSmsTemplatesResponseContent, error)

	// SetTemplates sets the SMS enrollment and verification templates.
	SetTemplates(ctx context.Context, request *managementv3.SetGuardianFactorSmsTemplatesRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorSmsTemplatesResponseContent, error)

	// GetTwilioProvider retrieves the Twilio configuration for the SMS factor.
	GetTwilioProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderSmsTwilioResponseContent, error)

	// SetTwilioProvider sets the Twilio configuration for the SMS factor.
	SetTwilioProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderSmsTwilioRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderSmsTwilioResponseContent, error)
}

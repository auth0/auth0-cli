//go:generate go tool mockgen -source=guardian_factor_phone.go -destination=mock/guardian_factor_phone_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianFactorPhoneAPIV3 is the V3 SDK interface for the phone MFA factor
// configuration (/guardian/factors/phone).
//
// Required scopes: `read:guardian_factors` for reads, `update:guardian_factors`
// for writes.
type GuardianFactorPhoneAPIV3 interface {
	// GetSelectedProvider retrieves the configured phone provider.
	GetSelectedProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderPhoneResponseContent, error)

	// SetProvider sets the phone provider.
	SetProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPhoneRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderPhoneResponseContent, error)

	// GetMessageTypes retrieves the enabled phone message types (sms/voice).
	GetMessageTypes(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorPhoneMessageTypesResponseContent, error)

	// SetMessageTypes sets the enabled phone message types (sms/voice).
	SetMessageTypes(ctx context.Context, request *managementv3.SetGuardianFactorPhoneMessageTypesRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorPhoneMessageTypesResponseContent, error)

	// GetTemplates retrieves the phone enrollment and verification templates.
	GetTemplates(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorPhoneTemplatesResponseContent, error)

	// SetTemplates sets the phone enrollment and verification templates.
	SetTemplates(ctx context.Context, request *managementv3.SetGuardianFactorPhoneTemplatesRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorPhoneTemplatesResponseContent, error)

	// GetTwilioProvider retrieves the Twilio configuration for the phone factor.
	GetTwilioProvider(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorsProviderPhoneTwilioResponseContent, error)

	// SetTwilioProvider sets the Twilio configuration for the phone factor.
	SetTwilioProvider(ctx context.Context, request *managementv3.SetGuardianFactorsProviderPhoneTwilioRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorsProviderPhoneTwilioResponseContent, error)
}

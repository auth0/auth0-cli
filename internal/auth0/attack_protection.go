package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/auth0/go-auth0/v2/management/option"
)

type AttackProtectionAPI interface {
	// GetBreachedPasswordDetection retrieves breached password detection settings.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_breached_password_detection
	GetBreachedPasswordDetection(ctx context.Context,
		opts ...management.RequestOption,
	) (bpd *management.BreachedPasswordDetection, err error)

	// UpdateBreachedPasswordDetection updates the breached password detection settings.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_breached_password_detection
	UpdateBreachedPasswordDetection(
		ctx context.Context, bpd *management.BreachedPasswordDetection,
		opts ...management.RequestOption,
	) (err error)

	// GetBruteForceProtection retrieves the brute force configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_brute_force_protection
	GetBruteForceProtection(
		ctx context.Context, opts ...management.RequestOption,
	) (bfp *management.BruteForceProtection, err error)

	// UpdateBruteForceProtection updates the brute force configuration.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_brute_force_protection
	UpdateBruteForceProtection(
		ctx context.Context, bfp *management.BruteForceProtection,
		opts ...management.RequestOption,
	) (err error)

	// GetSuspiciousIPThrottling retrieves the suspicious IP throttling configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_suspicious_ip_throttling
	GetSuspiciousIPThrottling(
		ctx context.Context, opts ...management.RequestOption,
	) (sit *management.SuspiciousIPThrottling, err error)

	// UpdateSuspiciousIPThrottling updates the suspicious IP throttling configuration.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_suspicious_ip_throttling
	UpdateSuspiciousIPThrottling(
		ctx context.Context, sit *management.SuspiciousIPThrottling,
		opts ...management.RequestOption,
	) (err error)
}

type AttackProtectionBotDetectionAPIV2 interface {
	// Get the Bot Detection configuration of tenant.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/attack-protection/get-bot-detection
	Get(ctx context.Context, opts ...option.RequestOption) (*managementv2.GetBotDetectionSettingsResponseContent, error)

	// Update the Bot Detection configuration of tenant.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/attack-protection/patch-bot-detection
	Update(ctx context.Context, request *managementv2.UpdateBotDetectionSettingsRequestContent, opts ...option.RequestOption) (*managementv2.UpdateBotDetectionSettingsResponseContent, error)
}

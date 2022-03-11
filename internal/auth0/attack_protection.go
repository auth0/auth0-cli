package auth0

import (
	"github.com/auth0/go-auth0/management"
)

type AttackProtectionAPI interface {
	// GetBreachedPasswordDetection retrieves breached password detection settings.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_breached_password_detection
	GetBreachedPasswordDetection(
		opts ...management.RequestOption,
	) (bpd *management.BreachedPasswordDetection, err error)

	// UpdateBreachedPasswordDetection updates the breached password detection settings.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_breached_password_detection
	UpdateBreachedPasswordDetection(
		bpd *management.BreachedPasswordDetection,
		opts ...management.RequestOption,
	) (err error)

	// GetBruteForceProtection retrieves the brute force configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_brute_force_protection
	GetBruteForceProtection(
		opts ...management.RequestOption,
	) (bfp *management.BruteForceProtection, err error)

	// UpdateBruteForceProtection updates the brute force configuration.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_brute_force_protection
	UpdateBruteForceProtection(
		bfp *management.BruteForceProtection,
		opts ...management.RequestOption,
	) (err error)

	// GetSuspiciousIPThrottling retrieves the suspicious IP throttling configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_suspicious_ip_throttling
	GetSuspiciousIPThrottling(
		opts ...management.RequestOption,
	) (sit *management.SuspiciousIPThrottling, err error)

	// UpdateSuspiciousIPThrottling updates the suspicious IP throttling configuration.
	//
	// Required scope: `update:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/patch_suspicious_ip_throttling
	UpdateSuspiciousIPThrottling(
		sit *management.SuspiciousIPThrottling,
		opts ...management.RequestOption,
	) (err error)
}

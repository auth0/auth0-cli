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

	// GetBruteForceProtection retrieves the brute force configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_brute_force_protection
	GetBruteForceProtection(
		opts ...management.RequestOption,
	) (bfp *management.BruteForceProtection, err error)

	// GetSuspiciousIPThrottling retrieves the suspicious IP throttling configuration.
	//
	// Required scope: `read:attack_protection`
	//
	// See: https://auth0.com/docs/api/management/v2#!/Attack_Protection/get_suspicious_ip_throttling
	GetSuspiciousIPThrottling(
		opts ...management.RequestOption,
	) (sit *management.SuspiciousIPThrottling, err error)
}

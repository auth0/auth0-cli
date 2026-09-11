//go:generate go tool mockgen -source=guardian_policy.go -destination=mock/guardian_policy_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianPolicyAPIV3 is the V3 SDK interface for the tenant-wide multi-factor
// authentication (MFA) policies endpoint (/guardian/policies).
type GuardianPolicyAPIV3 interface {
	// List retrieves the MFA policies configured for the tenant.
	//
	// Required scope: `read:mfa_policies`.
	List(ctx context.Context, opts ...option.RequestOption) (managementv3.ListGuardianPoliciesResponseContent, error)

	// Set replaces the MFA policies configured for the tenant.
	//
	// Required scope: `update:mfa_policies`.
	Set(ctx context.Context, request managementv3.SetGuardianPoliciesRequestContent, opts ...option.RequestOption) (managementv3.SetGuardianPoliciesResponseContent, error)
}

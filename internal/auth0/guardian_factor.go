//go:generate mockgen -source=guardian_factor.go -destination=mock/guardian_factor_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianFactorAPIV3 is the V3 SDK interface for enabling and disabling
// multi-factor authentication (MFA) factors (/guardian/factors).
type GuardianFactorAPIV3 interface {
	// List retrieves all MFA factors and their enabled/disabled status.
	//
	// Required scope: `read:guardian_factors`.
	List(ctx context.Context, opts ...option.RequestOption) ([]*managementv3.GuardianFactor, error)

	// Set enables or disables a single MFA factor.
	//
	// Required scope: `update:guardian_factors`.
	Set(ctx context.Context, name *managementv3.GuardianFactorNameEnum, request *managementv3.SetGuardianFactorRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorResponseContent, error)
}

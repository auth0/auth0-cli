//go:generate mockgen -source=guardian_factor_duo.go -destination=mock/guardian_factor_duo_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianFactorDuoAPIV3 is the V3 SDK interface for the Duo MFA factor
// settings (/guardian/factors/duo/settings).
//
// Required scopes: `read:guardian_factors` for reads, `update:guardian_factors`
// for writes.
type GuardianFactorDuoAPIV3 interface {
	// Get retrieves the Duo settings.
	Get(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetGuardianFactorDuoSettingsResponseContent, error)

	// Set replaces the Duo settings.
	Set(ctx context.Context, request *managementv3.SetGuardianFactorDuoSettingsRequestContent, opts ...option.RequestOption) (*managementv3.SetGuardianFactorDuoSettingsResponseContent, error)

	// Update partially updates the Duo settings.
	Update(ctx context.Context, request *managementv3.UpdateGuardianFactorDuoSettingsRequestContent, opts ...option.RequestOption) (*managementv3.UpdateGuardianFactorDuoSettingsResponseContent, error)
}

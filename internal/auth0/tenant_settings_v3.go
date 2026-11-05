//go:generate go tool mockgen -source=tenant_settings_v3.go -destination=mock/tenant_settings_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// TenantSettingsAPIV3 is the V3 SDK interface for the /tenants/settings
// endpoint. It exists because the v1 tenant model has no field for the
// authorization_response_iss_parameter_supported setting.
type TenantSettingsAPIV3 interface {
	// Get the tenant settings.
	//
	// Required scope: `read:tenant_settings`.
	Get(
		ctx context.Context,
		request *managementv3.GetTenantSettingsRequestParameters,
		opts ...option.RequestOption,
	) (*managementv3.GetTenantSettingsResponseContent, error)

	// Update the tenant settings.
	//
	// Required scope: `update:tenant_settings`.
	Update(
		ctx context.Context,
		request *managementv3.UpdateTenantSettingsRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.UpdateTenantSettingsResponseContent, error)
}

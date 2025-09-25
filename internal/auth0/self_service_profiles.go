//go:generate mockgen -source=self_service_profiles.go -destination=mock/self_service_profiles.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type SelfServiceProfileAPI interface {
	// Create a new sSelf Service Profiles.
	Create(ctx context.Context, p *management.SelfServiceProfile, opts ...management.RequestOption) error

	// List all Self Service Profiles.
	List(ctx context.Context, opts ...management.RequestOption) (p *management.SelfServiceProfileList, err error)

	// Read Self Service Profile details for a given profile ID.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (p *management.SelfServiceProfile, err error)

	// Update an existing Self Service Profile.
	Update(ctx context.Context, id string, p *management.SelfServiceProfile, opts ...management.RequestOption) error

	// Delete a Self Service Profile.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// GetCustomText retrieves text customizations for a given self-service profile, language and Self Service SSO Flow page.
	GetCustomText(ctx context.Context, id string, language string, page string, opts ...management.RequestOption) (payload map[string]interface{}, err error)
}

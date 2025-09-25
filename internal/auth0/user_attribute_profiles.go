//go:generate mockgen -source=user_attribute_profiles.go -destination=mock/user_attribute_profiles.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type UserAttributeProfilesAPI interface {
	Create(ctx context.Context, p *management.UserAttributeProfile, opts ...management.RequestOption) error

	List(ctx context.Context, opts ...management.RequestOption) (p *management.UserAttributeProfileList, err error)

	Read(ctx context.Context, id string, opts ...management.RequestOption) (p *management.UserAttributeProfile, err error)

	Update(ctx context.Context, id string, p *management.UserAttributeProfile, opts ...management.RequestOption) error

	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	ListTemplates(ctx context.Context, opts ...management.RequestOption) (p *management.UserAttributeProfileTemplateList, err error)

	GetTemplate(ctx context.Context, id string, opts ...management.RequestOption) (p *management.UserAttributeProfileTemplateItem, err error)
}

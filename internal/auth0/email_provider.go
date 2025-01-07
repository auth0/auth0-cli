//go:generate mockgen -source=email_provider.go -destination=mock/email_provider_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type EmailProviderAPI interface {
	// Create email provider.
	// See: https://auth0.com/docs/api/management/v2#!/Emails/post_provider
	Create(ctx context.Context, ep *management.EmailProvider, opts ...management.RequestOption) error

	// Update email provider.
	// See: https://auth0.com/docs/api/management/v2#!/Emails/patch_provider
	Update(ctx context.Context, ep *management.EmailProvider, opts ...management.RequestOption) (err error)

	// Read email provider details.
	// See: https://auth0.com/docs/api/management/v2#!/Emails/get_provider
	Read(ctx context.Context, opts ...management.RequestOption) (ep *management.EmailProvider, err error)

	// Delete the email provider.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Emails/delete_provider
	Delete(ctx context.Context, opts ...management.RequestOption) (err error)
}

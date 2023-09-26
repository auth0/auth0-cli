package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type EmailProviderAPI interface {
	// Read email provider details.
	// See: https://auth0.com/docs/api/management/v2#!/Emails/get_provider
	Read(ctx context.Context, opts ...management.RequestOption) (ep *management.EmailProvider, err error)
}

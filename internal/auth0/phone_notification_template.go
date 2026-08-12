//go:generate mockgen -source=phone_notification_template.go -destination=mock/phone_notification_template_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

type PhoneNotificationTemplateAPI interface {
	List(ctx context.Context, request *managementv3.ListPhoneTemplatesRequestParameters, opts ...option.RequestOption) (*managementv3.ListPhoneTemplatesResponseContent, error)
}

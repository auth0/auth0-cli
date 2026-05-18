//go:generate mockgen -source=phone_notification_template.go -destination=mock/phone_notification_template_mock.go -package=mock

package auth0

import (
	"context"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/auth0/go-auth0/v2/management/option"
)

type PhoneNotificationTemplateAPI interface {
	List(ctx context.Context, request *managementv2.ListPhoneTemplatesRequestParameters, opts ...option.RequestOption) (*managementv2.ListPhoneTemplatesResponseContent, error)
}

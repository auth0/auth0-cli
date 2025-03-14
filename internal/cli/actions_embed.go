package cli

import (
	_ "embed"
)

var (
	//go:embed data/action-template-post-login.js
	actionTemplatePostLogin string

	//go:embed data/action-template-credentials-exchange.js
	actionTemplateCredentialsExchange string

	//go:embed data/action-template-pre-user-registration.js
	actionTemplatePreUserRegistration string

	//go:embed data/action-template-post-user-registration.js
	actionTemplatePostUserRegistration string

	//go:embed data/action-template-post-change-password.js
	actionTemplatePostChangePassword string

	//go:embed data/action-template-send-phone-message.js
	actionTemplateSendPhoneMessage string

	//go:embed data/action-template-custom-email-provider.js
	actionTemplateCustomEmailProvider string

	//go:embed data/action-template-custom-phone-provider.js
	actionTemplateCustomPhoneProvider string

	//go:embed data/action-template-empty.js
	actionTemplateEmpty string
)

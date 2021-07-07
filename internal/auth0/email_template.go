package auth0

import "gopkg.in/auth0.v5/management"

type EmailTemplateAPI interface {
	// Retrieve an email template by pre-defined name.
	//
	// These names are `verify_email`, `reset_email`, `welcome_email`,
	// `blocked_account`, `stolen_credentials`, `enrollment_email`, and
	// `mfa_oob_code`.
	//
	// The names `change_password`, and `password_reset` are also supported for
	// legacy scenarios.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Email_Templates/get_email_templates_by_templateName
	Read(template string, opts ...management.RequestOption) (e *management.EmailTemplate, err error)

	// Modify an email template.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Email_Templates/patch_email_templates_by_templateName
	Update(template string, e *management.EmailTemplate, opts ...management.RequestOption) (err error)
}

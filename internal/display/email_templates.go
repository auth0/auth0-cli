package display

import (
	"strconv"

	"gopkg.in/auth0.v5/management"
)

type emailView struct {
	Template          string
	From              string
	Subject           string
	ResultURL         string
	ResultURLLifetime string
	Enabled           string
	raw               interface{}
}

func (v *emailView) AsTableHeader() []string {
	return []string{}
}

func (v *emailView) AsTableRow() []string {
	return []string{}
}

func (v *emailView) KeyValues() [][]string {
	return [][]string{
		{"TEMPLATE", v.Template},
		{"FROM", v.From},
		{"SUBJECT", v.Subject},
		{"RESULT URL", v.ResultURL},
		{"RESULT URL LIFETIME", v.ResultURLLifetime},
		{"ENABLED", v.Enabled},
	}
}

func (v *emailView) Object() interface{} {
	return v.raw
}

func (r *Renderer) EmailTemplateShow(email *management.EmailTemplate) {
	r.Heading("email template")
	r.Result(makeEmailTemplateView(email))
}

func (r *Renderer) EmailTemplateUpdate(email *management.EmailTemplate) {
	r.Heading("email template updated")
	r.Result(makeEmailTemplateView(email))
}

func makeEmailTemplateView(email *management.EmailTemplate) *emailView {
	return &emailView{
		Template:          emailTemplateFor(email.GetTemplate()),
		From:              email.GetFrom(),
		Subject:           email.GetSubject(),
		ResultURL:         email.GetResultURL(),
		ResultURLLifetime: strconv.Itoa(email.GetURLLifetimeInSecoonds()),
		Enabled:           boolean(email.GetEnabled()),
		raw:               email,
	}
}

func emailTemplateFor(v string) string {
	switch v {
	case "verify_email":
		return "Verification Email (using Link)"
	case "verify_email_by_code":
		return "Verification Email (using Code)"
	case "change_password":
		return "Change Password"
	case "welcome_email":
		return "Welcome Email"
	case "blocked_account":
		return "Blocked Account Email"
	case "stolen_credentials":
		return "Password Breach Alert"
	case "enrollment_email":
		return "Enroll in Multifactor Authentication"
	case "mfa_oob_code":
		return "Verification Code for Email MFA"
	case "user_invitation":
		return "User Invitation"
	default:
		return v
	}
}

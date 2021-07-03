package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

const (
	emailTemplateVerifyLink     = "verify-link"
	emailTemplateVerifyCode     = "verify-code"
	emailTemplateChangePassword = "change-password"
	emailTemplateWelcome        = "welcome"
	emailTemplateBlockedAccount = "blocked-account"
	emailTemplatePasswordBreach = "password-breach"
	emailTemplateMFAEnrollment  = "mfa-enrollment"
	emailTemplateMFACode        = "mfa-code"
	emailTemplateUserInvitation = "user-invitation"
)

var (
	emailTemplateTemplate = Argument{
		Name: "Template",
		Help: fmt.Sprintf("Template name. Can be '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s' or '%s'",
			emailTemplateVerifyLink,
			emailTemplateVerifyCode,
			emailTemplateChangePassword,
			emailTemplateWelcome,
			emailTemplateBlockedAccount,
			emailTemplatePasswordBreach,
			emailTemplateMFAEnrollment,
			emailTemplateMFACode,
			emailTemplateUserInvitation),
	}

	emailTemplateBody = Flag{
		Name:       "Body",
		LongForm:   "body",
		ShortForm:  "b",
		Help:       "Body of the email template.",
		IsRequired: true,
	}

	emailTemplateFrom = Flag{
		Name:         "From",
		LongForm:     "from",
		ShortForm:    "f",
		Help:         "Sender's 'from' email address.",
		AlwaysPrompt: true,
	}

	emailTemplateSubject = Flag{
		Name:         "Subject",
		LongForm:     "subject",
		ShortForm:    "s",
		Help:         "Subject line of the email.",
		AlwaysPrompt: true,
	}

	emailTemplateEnabled = Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Whether the template is enabled (true) or disabled (false).",
		AlwaysPrompt: true,
	}

	emailTemplateURL = Flag{
		Name:      "Result URL",
		LongForm:  "url",
		ShortForm: "u",
		Help:      "URL to redirect the user to after a successful action.",
	}

	emailTemplateLifetime = Flag{
		Name:      "Result URL Lifetime",
		LongForm:  "lifetime",
		ShortForm: "l",
		Help:      "Lifetime in seconds that the link within the email will be valid for.",
	}

	emailTemplateOptions = pickerOptions{
		{"Verification Email (using Link)", emailTemplateVerifyLink},
		{"Verification Email (using Code)", emailTemplateVerifyCode},
		{"Change Password", emailTemplateChangePassword},
		{"Welcome Email", emailTemplateWelcome, },
		{"Blocked Account Email", emailTemplateBlockedAccount},
		{"Password Breach Alert", emailTemplatePasswordBreach},
		{"Enroll in Multifactor Authentication", emailTemplateMFAEnrollment},
		{"Verification Code for Email MFA", emailTemplateMFACode},
		{"User Invitation", emailTemplateUserInvitation},
	}
)

func showEmailTemplateCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Template string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an email template",
		Long:  "Show an email template.",
		Example: `auth0 branding emails show <template>
auth0 branding emails show welcome`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := emailTemplateTemplate.Pick(cmd, &inputs.Template, cli.emailTemplatePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.Template = args[0]
			}

			var email *management.EmailTemplate

			if err := ansi.Waiting(func() error {
				var err error
				email, err = cli.api.EmailTemplate.Read(apiEmailTemplateFor(inputs.Template))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get the email template '%s': %w", inputs.Template, err)
			}

			cli.renderer.EmailTemplateShow(email)
			return nil
		},
	}

	return cmd
}

func updateEmailTemplateCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Template          string
		Body              string
		From              string
		Subject           string
		Enabled           bool
		ResultURL         string
		ResultURLLifetime int
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an email template",
		Long:  "Update an email template.",
		Example: `auth0 branding emails update <template>
auth0 branding emails update welcome`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.Template = args[0]
			} else {
				err := emailTemplateTemplate.Pick(cmd, &inputs.Template, cli.emailTemplatePickerOptions)
				if err != nil {
					return err
				}
			}

			var current *management.EmailTemplate
			err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.EmailTemplate.Read(apiEmailTemplateFor(inputs.Template))
				return err
			})
			if err != nil {
				return fmt.Errorf("Unable to get the email template '%s': %w", inputs.Template, err)
			}

			if err := emailTemplateFrom.AskU(cmd, &inputs.From, current.From); err != nil {
				return err
			}

			if err := emailTemplateSubject.AskU(cmd, &inputs.Subject, current.Subject); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			if err := emailTemplateBody.EditorPromptU(
				cmd,
				&inputs.Body,
				current.GetBody(),
				inputs.Template+".*.liquid",
				cli.emailTemplateEditorHint,
			); err != nil {
				return err
			}
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			if !ruleEnabled.IsSet(cmd) {
				inputs.Enabled = auth0.BoolValue(current.Enabled)
			}

			if err := ruleEnabled.AskBoolU(cmd, &inputs.Enabled, current.Enabled); err != nil {
				return err
			}

			if inputs.From == "" {
				inputs.From = current.GetFrom()
			}

			if inputs.Subject == "" {
				inputs.Subject = current.GetSubject()
			}

			template := apiEmailTemplateFor(inputs.Template)
			// Prepare email template payload for update. This will also be
			// re-hydrated by the SDK, which we'll use below during
			// display.
			emailTemplate := &management.EmailTemplate{
				Template: &template,
				Body: &inputs.Body,
				From: &inputs.From,
				Subject: &inputs.Subject,
				Enabled: &inputs.Enabled,
			}

			if inputs.ResultURL == "" {
				emailTemplate.ResultURL = current.ResultURL
			} else {
				emailTemplate.ResultURL = &inputs.ResultURL
			}

			if inputs.ResultURLLifetime == 0 {
				emailTemplate.URLLifetimeInSecoonds = current.URLLifetimeInSecoonds
			} else {
				emailTemplate.URLLifetimeInSecoonds = &inputs.ResultURLLifetime
			}

			if err = ansi.Waiting(func() error {
				return cli.api.EmailTemplate.Update(template, emailTemplate)
			}); err != nil {
				return err
			}

			cli.renderer.EmailTemplateUpdate(emailTemplate)
			return nil
		},
	}

	emailTemplateBody.RegisterStringU(cmd, &inputs.Body, "")
	emailTemplateFrom.RegisterStringU(cmd, &inputs.From, "")
	emailTemplateSubject.RegisterStringU(cmd, &inputs.Subject, "")
	emailTemplateEnabled.RegisterBoolU(cmd, &inputs.Enabled, true)
	emailTemplateURL.RegisterStringU(cmd, &inputs.ResultURL, "")
	emailTemplateLifetime.RegisterIntU(cmd, &inputs.ResultURLLifetime, 0)

	return cmd
}

func (c *cli) emailTemplateEditorHint() {
	c.renderer.Infof("%s once you close the editor, the email template will be saved. To cancel, CTRL+C.", ansi.Faint("Hint:"))
}

func (c *cli) emailTemplatePickerOptions() (pickerOptions, error) {
	return emailTemplateOptions, nil
}

func apiEmailTemplateFor(v string) string {
	switch v {
	case emailTemplateVerifyLink:
		return "verify_email"
	case emailTemplateVerifyCode:
		return "verify_email_by_code"
	case emailTemplateChangePassword:
		return "reset_email"
	case emailTemplateWelcome:
		return "welcome_email"
	case emailTemplateBlockedAccount:
		return "blocked_account"
	case emailTemplatePasswordBreach:
		return "stolen_credentials"
	case emailTemplateMFAEnrollment:
		return "enrollment_email"
	case emailTemplateMFACode:
		return "mfa_oob_code"
	case emailTemplateUserInvitation:
		return "user_invitation"
	default:
		return v
	}
}

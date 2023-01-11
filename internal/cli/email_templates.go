package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
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
		{"Welcome Email", emailTemplateWelcome},
		{"Blocked Account Email", emailTemplateBlockedAccount},
		{"Password Breach Alert", emailTemplatePasswordBreach},
		{"Enroll in Multifactor Authentication", emailTemplateMFAEnrollment},
		{"Verification Code for Email MFA", emailTemplateMFACode},
		{"User Invitation", emailTemplateUserInvitation},
	}
)

func emailTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage custom email templates",
		Long:  "Manage custom email templates. This requires a custom email provider to be configured for the tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showEmailTemplateCmd(cli))
	cmd.AddCommand(updateEmailTemplateCmd(cli))
	return cmd
}

func showEmailTemplateCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Template string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an email template",
		Long:  "Display information about an email template.",
		Example: `  auth0 email templates show
  auth0 email templates show <template>
  auth0 email templates show welcome`,
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
		Long: "Update an email template.\n\n" +
			"To update interactively, use `auth0 email templates update` with no arguments.\n\n" +
			"To update non-interactively, supply the template name and other information " +
			"through the flags.",
		Example: `  auth0 email templates update
  auth0 email templates update <template>
  auth0 email templates update <template> --json
  auth0 email templates update welcome --enabled true
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100 --subject "Welcome"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100 --subject "Welcome" --url "https://example.com"
  auth0 email templates update welcome -e true -b "$(cat path/to/body.html)" -f "welcome@example.com" -l 6100 -s "Welcome" -u "https://example.com" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.Template = args[0]
			} else {
				err := emailTemplateTemplate.Pick(cmd, &inputs.Template, cli.emailTemplatePickerOptions)
				if err != nil {
					return err
				}
			}

			var oldTemplate *management.EmailTemplate
			err := ansi.Waiting(func() (err error) {
				oldTemplate, err = cli.api.EmailTemplate.Read(apiEmailTemplateFor(inputs.Template))
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to get the email template '%s': %w", inputs.Template, err)
			}

			if err := emailTemplateFrom.AskU(cmd, &inputs.From, oldTemplate.From); err != nil {
				return err
			}
			if err := emailTemplateSubject.AskU(cmd, &inputs.Subject, oldTemplate.Subject); err != nil {
				return err
			}

			if err := emailTemplateBody.OpenEditorU(
				cmd,
				&inputs.Body,
				oldTemplate.GetBody(),
				inputs.Template+".*.liquid",
				cli.emailTemplateEditorHint,
			); err != nil {
				return fmt.Errorf("failed to capture input from the editor: %w", err)
			}

			if err := emailTemplateEnabled.AskBoolU(cmd, &inputs.Enabled, oldTemplate.Enabled); err != nil {
				return err
			}

			template := apiEmailTemplateFor(inputs.Template)
			emailTemplate := &management.EmailTemplate{
				Enabled:  &inputs.Enabled,
				Template: &template,
			}
			if inputs.Body != "" {
				emailTemplate.Body = &inputs.Body
			}
			if inputs.From != "" {
				emailTemplate.From = &inputs.From
			}
			if inputs.Subject != "" {
				emailTemplate.Subject = &inputs.Subject
			}
			if inputs.ResultURL != "" {
				emailTemplate.ResultURL = &inputs.ResultURL
			}
			if inputs.ResultURLLifetime != 0 {
				emailTemplate.URLLifetimeInSecoonds = &inputs.ResultURLLifetime
			}

			if err = ansi.Waiting(func() error {
				return cli.api.EmailTemplate.Update(template, emailTemplate)
			}); err != nil {
				return fmt.Errorf("failed to update the email template '%s': %w", inputs.Template, err)
			}

			cli.renderer.EmailTemplateUpdate(emailTemplate)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	emailTemplateBody.RegisterStringU(cmd, &inputs.Body, "")
	emailTemplateFrom.RegisterStringU(cmd, &inputs.From, "")
	emailTemplateSubject.RegisterStringU(cmd, &inputs.Subject, "")
	emailTemplateEnabled.RegisterBoolU(cmd, &inputs.Enabled, true)
	emailTemplateURL.RegisterStringU(cmd, &inputs.ResultURL, "")
	emailTemplateLifetime.RegisterIntU(cmd, &inputs.ResultURLLifetime, 0)

	return cmd
}

func (c *cli) emailTemplateEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the email template will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
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

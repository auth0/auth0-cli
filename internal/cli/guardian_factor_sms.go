package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
)

func guardianFactorSmsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sms",
		Short: "Manage the SMS multi-factor authentication factor (legacy)",
		Long: "Manage the SMS MFA factor provider, templates and Twilio configuration.\n\n" +
			"These are legacy endpoints. Tenants on the unified phone experience must manage SMS " +
			"delivery from the Dashboard (Branding > Phone Provider); they are not available to Management API tokens.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showGuardianSmsProviderCmd(cli))
	cmd.AddCommand(setGuardianSmsProviderCmd(cli))
	cmd.AddCommand(showGuardianSmsTemplatesCmd(cli))
	cmd.AddCommand(setGuardianSmsTemplatesCmd(cli))
	cmd.AddCommand(showGuardianSmsTwilioCmd(cli))
	cmd.AddCommand(setGuardianSmsTwilioCmd(cli))

	return cmd
}

func showGuardianSmsProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-provider",
		Args:  cobra.NoArgs,
		Short: "Show the SMS provider (legacy)",
		Long: "Display the configured SMS MFA provider.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms show-provider --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderSmsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.GetSelectedProvider(cmd.Context())
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to read SMS provider: %w", err))
			}

			cli.renderer.GuardianDetail("sms provider", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianSmsProviderCmd(cli *cli) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "set-provider",
		Args:  cobra.NoArgs,
		Short: "Set the SMS provider (legacy)",
		Long: "Set the SMS MFA provider. One of: auth0, twilio, phone-message-hook.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms set-provider --provider twilio
  auth0 guardian factors sms set-provider --provider auth0 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianProvider.Select(cmd, &provider, guardianSmsProviderOptions, nil); err != nil {
				return err
			}

			value, err := managementv3.NewGuardianFactorsProviderSmsProviderEnumFromString(provider)
			if err != nil {
				return fmt.Errorf("invalid provider %q: valid values are auth0, twilio, phone-message-hook", provider)
			}

			body := &managementv3.SetGuardianFactorsProviderSmsRequestContent{Provider: value}

			var resp *managementv3.SetGuardianFactorsProviderSmsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.SetProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set SMS provider: %w", err))
			}

			cli.renderer.GuardianDetail("sms provider updated", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianProvider.RegisterString(cmd, &provider, "")

	return cmd
}

func showGuardianSmsTemplatesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-templates",
		Args:  cobra.NoArgs,
		Short: "Show the SMS templates (legacy)",
		Long: "Display the SMS enrollment and verification message templates.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"templates from the Dashboard (Branding > Phone Templates); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms show-templates --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorSmsTemplatesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.GetTemplates(cmd.Context())
				return err
			}); err != nil {
				if !isEmptyResponseErr(err) {
					return guardianLegacyPhoneHint(fmt.Errorf("failed to read SMS templates: %w", err))
				}
				// No templates configured: the endpoint returns an empty body.
				resp = &managementv3.GetGuardianFactorSmsTemplatesResponseContent{}
			}

			cli.renderer.GuardianDetail("sms templates", [][]string{
				{"ENROLLMENT MESSAGE", resp.GetEnrollmentMessage()},
				{"VERIFICATION MESSAGE", resp.GetVerificationMessage()},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianSmsTemplatesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		EnrollmentMessage   string
		VerificationMessage string
	}

	cmd := &cobra.Command{
		Use:   "set-templates",
		Args:  cobra.NoArgs,
		Short: "Set the SMS templates (legacy)",
		Long: "Set the SMS enrollment and verification message templates.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"templates from the Dashboard (Branding > Phone Templates); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms set-templates \
    --enrollment-message "Your verification code is {{code}}" \
    --verification-message "Your verification code is {{code}}"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianEnrollmentMessage.Ask(cmd, &inputs.EnrollmentMessage, nil); err != nil {
				return err
			}
			if err := guardianVerificationMessage.Ask(cmd, &inputs.VerificationMessage, nil); err != nil {
				return err
			}

			body := &managementv3.SetGuardianFactorSmsTemplatesRequestContent{
				EnrollmentMessage:   inputs.EnrollmentMessage,
				VerificationMessage: inputs.VerificationMessage,
			}

			var resp *managementv3.SetGuardianFactorSmsTemplatesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.SetTemplates(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set SMS templates: %w", err))
			}

			cli.renderer.GuardianDetail("sms templates updated", [][]string{
				{"ENROLLMENT MESSAGE", resp.GetEnrollmentMessage()},
				{"VERIFICATION MESSAGE", resp.GetVerificationMessage()},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianEnrollmentMessage.RegisterString(cmd, &inputs.EnrollmentMessage, "")
	guardianVerificationMessage.RegisterString(cmd, &inputs.VerificationMessage, "")

	return cmd
}

func showGuardianSmsTwilioCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-twilio",
		Args:  cobra.NoArgs,
		Short: "Show the SMS Twilio configuration (legacy)",
		Long: "Display the Twilio configuration for the SMS MFA factor.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms show-twilio --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderSmsTwilioResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.GetTwilioProvider(cmd.Context())
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to read SMS Twilio configuration: %w", err))
			}

			cli.renderer.GuardianDetail("sms twilio configuration", [][]string{
				{"FROM", resp.GetFrom()},
				{"MESSAGING SERVICE SID", resp.GetMessagingServiceSid()},
				{"SID", resp.GetSid()},
				{"AUTH TOKEN", display.MaskSecret(resp.GetAuthToken())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianSmsTwilioCmd(cli *cli) *cobra.Command {
	var inputs struct {
		From                string
		MessagingServiceSid string
		Sid                 string
		AuthToken           string
	}

	cmd := &cobra.Command{
		Use:   "set-twilio",
		Args:  cobra.NoArgs,
		Short: "Set the SMS Twilio configuration (legacy)",
		Long: "Set the Twilio configuration for the SMS MFA factor.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage SMS " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors sms set-twilio --sid AC... --auth-token <token> --from "+14155550100"
  auth0 guardian factors sms set-twilio --sid AC... --auth-token <token> --messaging-service-sid MG...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := &managementv3.SetGuardianFactorsProviderSmsTwilioRequestContent{}
			if inputs.From != "" {
				body.From = &inputs.From
			}
			if inputs.MessagingServiceSid != "" {
				body.MessagingServiceSid = &inputs.MessagingServiceSid
			}
			if inputs.Sid != "" {
				body.Sid = &inputs.Sid
			}
			if inputs.AuthToken != "" {
				body.AuthToken = &inputs.AuthToken
			}

			var resp *managementv3.SetGuardianFactorsProviderSmsTwilioResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorSms.SetTwilioProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set SMS Twilio configuration: %w", err))
			}

			cli.renderer.GuardianDetail("sms twilio configuration updated", [][]string{
				{"FROM", resp.GetFrom()},
				{"MESSAGING SERVICE SID", resp.GetMessagingServiceSid()},
				{"SID", resp.GetSid()},
				{"AUTH TOKEN", display.MaskSecret(resp.GetAuthToken())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianTwilioFrom.RegisterString(cmd, &inputs.From, "")
	guardianTwilioMessagingServiceSid.RegisterString(cmd, &inputs.MessagingServiceSid, "")
	guardianTwilioSid.RegisterString(cmd, &inputs.Sid, "")
	guardianTwilioAuthToken.RegisterString(cmd, &inputs.AuthToken, "")

	return cmd
}

package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
)

func guardianFactorPhoneCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "phone",
		Short: "Manage the phone multi-factor authentication factor",
		Long:  "Manage the phone MFA factor provider, message types and templates.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showGuardianPhoneProviderCmd(cli))
	cmd.AddCommand(setGuardianPhoneProviderCmd(cli))
	cmd.AddCommand(showGuardianPhoneMessageTypesCmd(cli))
	cmd.AddCommand(setGuardianPhoneMessageTypesCmd(cli))
	cmd.AddCommand(showGuardianPhoneTemplatesCmd(cli))
	cmd.AddCommand(setGuardianPhoneTemplatesCmd(cli))
	cmd.AddCommand(showGuardianPhoneTwilioCmd(cli))
	cmd.AddCommand(setGuardianPhoneTwilioCmd(cli))

	return cmd
}

func registerGuardianJSONFlags(cli *cli, cmd *cobra.Command) {
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
}

func showGuardianPhoneProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-provider",
		Args:  cobra.NoArgs,
		Short: "Show the phone provider (legacy)",
		Long: "Display the configured phone MFA provider.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone show-provider
  auth0 guardian factors phone show-provider --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderPhoneResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.GetSelectedProvider(cmd.Context())
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to read phone provider: %w", err))
			}

			cli.renderer.GuardianDetail("phone provider", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPhoneProviderCmd(cli *cli) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "set-provider",
		Args:  cobra.NoArgs,
		Short: "Set the phone provider (legacy)",
		Long: "Set the phone MFA provider. One of: auth0, twilio, phone-message-hook.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone set-provider --provider twilio
  auth0 guardian factors phone set-provider --provider auth0 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianProvider.Select(cmd, &provider, guardianSmsProviderOptions, nil); err != nil {
				return err
			}

			value, err := managementv3.NewGuardianFactorsProviderSmsProviderEnumFromString(provider)
			if err != nil {
				return fmt.Errorf("invalid provider %q: valid values are auth0, twilio, phone-message-hook", provider)
			}

			body := &managementv3.SetGuardianFactorsProviderPhoneRequestContent{Provider: value}

			var resp *managementv3.SetGuardianFactorsProviderPhoneResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.SetProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set phone provider: %w", err))
			}

			cli.renderer.GuardianDetail("phone provider updated", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianProvider.RegisterString(cmd, &provider, "")

	return cmd
}

func showGuardianPhoneMessageTypesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show-message-types",
		Args:    cobra.NoArgs,
		Short:   "Show the phone message types",
		Long:    "Display the enabled phone message types (sms, voice).",
		Example: `  auth0 guardian factors phone show-message-types --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorPhoneMessageTypesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.GetMessageTypes(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read phone message types: %w", err)
			}

			cli.renderer.GuardianDetail("phone message types", [][]string{
				{"MESSAGE TYPES", messageTypesForDisplay(resp.GetMessageTypes())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPhoneMessageTypesCmd(cli *cli) *cobra.Command {
	var messageTypes []string

	cmd := &cobra.Command{
		Use:   "set-message-types",
		Args:  cobra.NoArgs,
		Short: "Set the phone message types",
		Long:  "Set the enabled phone message types. Supported values: sms, voice.",
		Example: `  auth0 guardian factors phone set-message-types --message-type sms --message-type voice
  auth0 guardian factors phone set-message-types --message-type sms --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !guardianMessageType.IsSet(cmd) && canPrompt(cmd) {
				if err := guardianMessageType.PickMany(cmd, &messageTypes, staticPickerOptions(guardianMessageTypeOptions)); err != nil {
					return err
				}
			}

			types := make([]managementv3.GuardianFactorPhoneFactorMessageTypeEnum, 0, len(messageTypes))
			for _, t := range messageTypes {
				value, err := managementv3.NewGuardianFactorPhoneFactorMessageTypeEnumFromString(t)
				if err != nil {
					return fmt.Errorf("invalid message type %q: valid values are sms, voice", t)
				}
				types = append(types, value)
			}

			body := &managementv3.SetGuardianFactorPhoneMessageTypesRequestContent{MessageTypes: types}

			var resp *managementv3.SetGuardianFactorPhoneMessageTypesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.SetMessageTypes(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set phone message types: %w", err)
			}

			cli.renderer.GuardianDetail("phone message types updated", [][]string{
				{"MESSAGE TYPES", messageTypesForDisplay(resp.GetMessageTypes())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianMessageType.RegisterStringSlice(cmd, &messageTypes, nil)

	return cmd
}

func showGuardianPhoneTemplatesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-templates",
		Args:  cobra.NoArgs,
		Short: "Show the phone templates (legacy)",
		Long: "Display the phone enrollment and verification message templates.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"templates from the Dashboard (Branding > Phone Templates); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone show-templates --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorPhoneTemplatesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.GetTemplates(cmd.Context())
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to read phone templates: %w", err))
			}

			cli.renderer.GuardianDetail("phone templates", [][]string{
				{"ENROLLMENT MESSAGE", resp.GetEnrollmentMessage()},
				{"VERIFICATION MESSAGE", resp.GetVerificationMessage()},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPhoneTemplatesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		EnrollmentMessage   string
		VerificationMessage string
	}

	cmd := &cobra.Command{
		Use:   "set-templates",
		Args:  cobra.NoArgs,
		Short: "Set the phone templates (legacy)",
		Long: "Set the phone enrollment and verification message templates.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"templates from the Dashboard (Branding > Phone Templates); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone set-templates \
    --enrollment-message "Your verification code is {{code}}" \
    --verification-message "Your verification code is {{code}}"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianEnrollmentMessage.Ask(cmd, &inputs.EnrollmentMessage, nil); err != nil {
				return err
			}
			if err := guardianVerificationMessage.Ask(cmd, &inputs.VerificationMessage, nil); err != nil {
				return err
			}

			body := &managementv3.SetGuardianFactorPhoneTemplatesRequestContent{
				EnrollmentMessage:   inputs.EnrollmentMessage,
				VerificationMessage: inputs.VerificationMessage,
			}

			var resp *managementv3.SetGuardianFactorPhoneTemplatesResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.SetTemplates(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set phone templates: %w", err))
			}

			cli.renderer.GuardianDetail("phone templates updated", [][]string{
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

func showGuardianPhoneTwilioCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-twilio",
		Args:  cobra.NoArgs,
		Short: "Show the phone Twilio configuration (legacy)",
		Long: "Display the Twilio configuration for the phone MFA factor.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone show-twilio --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderPhoneTwilioResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.GetTwilioProvider(cmd.Context())
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to read phone Twilio configuration: %w", err))
			}

			cli.renderer.GuardianDetail("phone twilio configuration", [][]string{
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

func setGuardianPhoneTwilioCmd(cli *cli) *cobra.Command {
	var inputs struct {
		From                string
		MessagingServiceSid string
		Sid                 string
		AuthToken           string
	}

	cmd := &cobra.Command{
		Use:   "set-twilio",
		Args:  cobra.NoArgs,
		Short: "Set the phone Twilio configuration (legacy)",
		Long: "Set the Twilio configuration for the phone MFA factor.\n\n" +
			"This is a legacy endpoint. Tenants on the unified phone experience must manage phone " +
			"delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.",
		Example: `  auth0 guardian factors phone set-twilio --sid AC... --auth-token <token> --from "+14155550100"
  auth0 guardian factors phone set-twilio --sid AC... --auth-token <token> --messaging-service-sid MG...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := &managementv3.SetGuardianFactorsProviderPhoneTwilioRequestContent{}
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

			var resp *managementv3.SetGuardianFactorsProviderPhoneTwilioResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPhone.SetTwilioProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return guardianLegacyPhoneHint(fmt.Errorf("failed to set phone Twilio configuration: %w", err))
			}

			cli.renderer.GuardianDetail("phone twilio configuration updated", [][]string{
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

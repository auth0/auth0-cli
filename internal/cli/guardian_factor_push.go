package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
)

func guardianFactorPushCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Manage the push-notification multi-factor authentication factor",
		Long:  "Manage the push-notification MFA factor provider and its APNs, FCM, FCM v1 and SNS configuration.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showGuardianPushProviderCmd(cli))
	cmd.AddCommand(setGuardianPushProviderCmd(cli))
	cmd.AddCommand(showGuardianPushApnsCmd(cli))
	cmd.AddCommand(setGuardianPushApnsCmd(cli))
	cmd.AddCommand(updateGuardianPushApnsCmd(cli))
	cmd.AddCommand(setGuardianPushFcmCmd(cli))
	cmd.AddCommand(updateGuardianPushFcmCmd(cli))
	cmd.AddCommand(setGuardianPushFcmv1Cmd(cli))
	cmd.AddCommand(updateGuardianPushFcmv1Cmd(cli))
	cmd.AddCommand(showGuardianPushSnsCmd(cli))
	cmd.AddCommand(setGuardianPushSnsCmd(cli))
	cmd.AddCommand(updateGuardianPushSnsCmd(cli))

	return cmd
}

func showGuardianPushProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show-provider",
		Args:    cobra.NoArgs,
		Short:   "Show the push-notification provider",
		Long:    "Display the configured push-notification MFA provider.",
		Example: `  auth0 guardian factors push show-provider --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderPushNotificationResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.GetSelectedProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read push provider: %w", err)
			}

			cli.renderer.GuardianDetail("push provider", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPushProviderCmd(cli *cli) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "set-provider",
		Args:  cobra.NoArgs,
		Short: "Set the push-notification provider",
		Long:  "Set the push-notification MFA provider. One of: guardian, sns, direct.",
		Example: `  auth0 guardian factors push set-provider --provider guardian
  auth0 guardian factors push set-provider --provider sns --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianProvider.Select(cmd, &provider, guardianPushProviderOptions, nil); err != nil {
				return err
			}

			if provider == "" {
				return fmt.Errorf("--provider is required: valid values are guardian, sns, direct")
			}

			value, err := managementv3.NewGuardianFactorsProviderPushNotificationProviderDataEnumFromString(provider)
			if err != nil {
				return fmt.Errorf("invalid provider %q: valid values are guardian, sns, direct", provider)
			}

			body := &managementv3.SetGuardianFactorsProviderPushNotificationRequestContent{Provider: value}

			var resp *managementv3.SetGuardianFactorsProviderPushNotificationResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.SetProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set push provider: %w", err)
			}

			cli.renderer.GuardianDetail("push provider updated", [][]string{
				{"PROVIDER", string(resp.GetProvider())},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianProvider.RegisterString(cmd, &provider, "")

	return cmd
}

func guardianApnsRows(sandbox bool, bundleID string) [][]string {
	return [][]string{
		{"BUNDLE ID", orDashCLI(bundleID)},
		{"SANDBOX", fmt.Sprintf("%t", sandbox)},
	}
}

// orDashCLI mirrors the display package's dash placeholder for empty strings so
// the command layer can compose rows without leaking an empty cell.
func orDashCLI(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func showGuardianPushApnsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show-apns",
		Args:    cobra.NoArgs,
		Short:   "Show the APNs configuration",
		Long:    "Display the Apple Push Notification service (APNs) configuration.",
		Example: `  auth0 guardian factors push show-apns --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderApnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.GetApnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read APNs configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push apns configuration", guardianApnsRows(resp.GetSandbox(), resp.GetBundleID()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPushApnsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		BundleID string
		Sandbox  bool
		P12      string
	}

	cmd := &cobra.Command{
		Use:     "set-apns",
		Args:    cobra.NoArgs,
		Short:   "Set the APNs configuration",
		Long:    "Replace the Apple Push Notification service (APNs) configuration.",
		Example: `  auth0 guardian factors push set-apns --bundle-id com.example.app --sandbox --p12 <base64-cert>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.BundleID == "" && inputs.P12 == "" && !cmd.Flags().Changed("sandbox") {
				return fmt.Errorf(
					"set replaces the entire APNs configuration, so pass at least one of --bundle-id, " +
						"--p12 or --sandbox. To change a single field, use 'auth0 guardian factors push update-apns'",
				)
			}

			body := &managementv3.SetGuardianFactorsProviderPushNotificationApnsRequestContent{}
			if inputs.BundleID != "" {
				body.BundleID = &inputs.BundleID
			}
			if cmd.Flags().Changed("sandbox") {
				body.Sandbox = &inputs.Sandbox
			}
			if inputs.P12 != "" {
				body.P12 = &inputs.P12
			}

			var resp *managementv3.SetGuardianFactorsProviderPushNotificationApnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.SetApnsProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set APNs configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push apns configuration updated", guardianApnsRows(resp.GetSandbox(), resp.GetBundleID()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianApnsBundleID.RegisterString(cmd, &inputs.BundleID, "")
	guardianApnsSandbox.RegisterBool(cmd, &inputs.Sandbox, false)
	guardianApnsP12.RegisterString(cmd, &inputs.P12, "")

	return cmd
}

func updateGuardianPushApnsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		BundleID string
		Sandbox  bool
		P12      string
	}

	cmd := &cobra.Command{
		Use:   "update-apns",
		Args:  cobra.NoArgs,
		Short: "Update the APNs configuration",
		Long: "Partially update the Apple Push Notification service (APNs) configuration. Only the fields " +
			"you provide are changed; the rest keep their current values. Run without flags to be prompted " +
			"for each field, pre-filled with the current value (leave the .p12 blank to keep it unchanged).",
		Example: `  auth0 guardian factors push update-apns --sandbox`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fetch the current configuration so the interactive prompts can
			// default to the existing values (the .p12 is never pre-filled).
			var current *managementv3.GetGuardianFactorsProviderApnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.GuardianFactorPush.GetApnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read APNs configuration: %w", err)
			}

			currentBundleID := current.GetBundleID()
			if err := guardianApnsBundleID.AskU(cmd, &inputs.BundleID, &currentBundleID); err != nil {
				return err
			}
			if !guardianApnsSandbox.IsSet(cmd) {
				inputs.Sandbox = current.GetSandbox()
			}
			currentSandbox := current.GetSandbox()
			if err := guardianApnsSandbox.AskBoolU(cmd, &inputs.Sandbox, &currentSandbox); err != nil {
				return err
			}
			if err := guardianApnsP12.AskU(cmd, &inputs.P12, nil); err != nil {
				return err
			}

			body := &managementv3.UpdateGuardianFactorsProviderPushNotificationApnsRequestContent{
				Sandbox: &inputs.Sandbox,
			}
			if inputs.BundleID != "" {
				body.BundleID = &inputs.BundleID
			}
			if inputs.P12 != "" {
				body.P12 = &inputs.P12
			}

			if err := ansi.Waiting(func() (err error) {
				_, err = cli.apiv3.GuardianFactorPush.UpdateApnsProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update APNs configuration: %w", err)
			}

			// The PATCH response only echoes the fields that were sent, so
			// re-fetch the full configuration to render the complete state.
			var resp *managementv3.GetGuardianFactorsProviderApnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.GetApnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read APNs configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push apns configuration updated", guardianApnsRows(resp.GetSandbox(), resp.GetBundleID()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianApnsBundleID.RegisterString(cmd, &inputs.BundleID, "")
	guardianApnsSandbox.RegisterBool(cmd, &inputs.Sandbox, false)
	guardianApnsP12.RegisterString(cmd, &inputs.P12, "")

	return cmd
}

func setGuardianPushFcmCmd(cli *cli) *cobra.Command {
	var serverKey string

	cmd := &cobra.Command{
		Use:     "set-fcm",
		Args:    cobra.NoArgs,
		Short:   "Set the FCM (legacy) configuration",
		Long:    "Replace the Google FCM (legacy) push-notification configuration.",
		Example: `  auth0 guardian factors push set-fcm --server-key <server-key>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianFcmServerKey.Ask(cmd, &serverKey, nil); err != nil {
				return err
			}

			body := &managementv3.SetGuardianFactorsProviderPushNotificationFcmRequestContent{}
			if serverKey != "" {
				body.ServerKey = &serverKey
			}

			var resp managementv3.SetGuardianFactorsProviderPushNotificationFcmResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.SetFcmProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set FCM configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push fcm configuration updated", [][]string{
				{"SERVER KEY", display.MaskSecret(serverKey)},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianFcmServerKey.RegisterString(cmd, &serverKey, "")

	return cmd
}

func updateGuardianPushFcmCmd(cli *cli) *cobra.Command {
	var serverKey string

	cmd := &cobra.Command{
		Use:     "update-fcm",
		Args:    cobra.NoArgs,
		Short:   "Update the FCM (legacy) configuration",
		Long:    "Partially update the Google FCM (legacy) push-notification configuration.",
		Example: `  auth0 guardian factors push update-fcm --server-key <server-key>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianFcmServerKey.Ask(cmd, &serverKey, nil); err != nil {
				return err
			}

			body := &managementv3.UpdateGuardianFactorsProviderPushNotificationFcmRequestContent{}
			if serverKey != "" {
				body.ServerKey = &serverKey
			}

			var resp managementv3.UpdateGuardianFactorsProviderPushNotificationFcmResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.UpdateFcmProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update FCM configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push fcm configuration updated", [][]string{
				{"SERVER KEY", display.MaskSecret(serverKey)},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianFcmServerKey.RegisterString(cmd, &serverKey, "")

	return cmd
}

func setGuardianPushFcmv1Cmd(cli *cli) *cobra.Command {
	var serverCredentials string

	cmd := &cobra.Command{
		Use:     "set-fcmv1",
		Args:    cobra.NoArgs,
		Short:   "Set the FCM v1 configuration",
		Long:    "Replace the Google FCM v1 push-notification configuration.",
		Example: `  auth0 guardian factors push set-fcmv1 --server-credentials "$(cat service-account.json)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianFcmServerCredentials.Ask(cmd, &serverCredentials, nil); err != nil {
				return err
			}

			body := &managementv3.SetGuardianFactorsProviderPushNotificationFcmv1RequestContent{}
			if serverCredentials != "" {
				body.ServerCredentials = &serverCredentials
			}

			var resp managementv3.SetGuardianFactorsProviderPushNotificationFcmv1ResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.SetFcmv1Provider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set FCM v1 configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push fcmv1 configuration updated", [][]string{
				{"SERVER CREDENTIALS", display.MaskSecret(serverCredentials)},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianFcmServerCredentials.RegisterString(cmd, &serverCredentials, "")

	return cmd
}

func updateGuardianPushFcmv1Cmd(cli *cli) *cobra.Command {
	var serverCredentials string

	cmd := &cobra.Command{
		Use:     "update-fcmv1",
		Args:    cobra.NoArgs,
		Short:   "Update the FCM v1 configuration",
		Long:    "Partially update the Google FCM v1 push-notification configuration.",
		Example: `  auth0 guardian factors push update-fcmv1 --server-credentials "$(cat service-account.json)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianFcmServerCredentials.Ask(cmd, &serverCredentials, nil); err != nil {
				return err
			}

			body := &managementv3.UpdateGuardianFactorsProviderPushNotificationFcmv1RequestContent{}
			if serverCredentials != "" {
				body.ServerCredentials = &serverCredentials
			}

			var resp managementv3.UpdateGuardianFactorsProviderPushNotificationFcmv1ResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.UpdateFcmv1Provider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update FCM v1 configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push fcmv1 configuration updated", [][]string{
				{"SERVER CREDENTIALS", display.MaskSecret(serverCredentials)},
			}, resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianFcmServerCredentials.RegisterString(cmd, &serverCredentials, "")

	return cmd
}

func guardianSnsRows(accessKeyID, secretAccessKey, region, apnsArn, gcmArn string) [][]string {
	return [][]string{
		{"AWS ACCESS KEY ID", orDashCLI(accessKeyID)},
		{"AWS SECRET ACCESS KEY", display.MaskSecret(secretAccessKey)},
		{"AWS REGION", orDashCLI(region)},
		{"APNS PLATFORM APPLICATION ARN", orDashCLI(apnsArn)},
		{"GCM PLATFORM APPLICATION ARN", orDashCLI(gcmArn)},
	}
}

func showGuardianPushSnsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show-sns",
		Args:    cobra.NoArgs,
		Short:   "Show the SNS configuration",
		Long:    "Display the Amazon SNS push-notification configuration.",
		Example: `  auth0 guardian factors push show-sns --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorsProviderSnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.GetSnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read SNS configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push sns configuration", guardianSnsRows(
				resp.GetAwsAccessKeyID(),
				resp.GetAwsSecretAccessKey(),
				resp.GetAwsRegion(),
				resp.GetSnsApnsPlatformApplicationArn(),
				resp.GetSnsGcmPlatformApplicationArn(),
			), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianPushSnsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		AccessKeyID     string
		SecretAccessKey string
		Region          string
		ApnsArn         string
		GcmArn          string
	}

	cmd := &cobra.Command{
		Use:   "set-sns",
		Args:  cobra.NoArgs,
		Short: "Set the SNS configuration",
		Long:  "Replace the Amazon SNS push-notification configuration.",
		Example: `  auth0 guardian factors push set-sns \
    --aws-access-key-id <id> --aws-secret-access-key <secret> --aws-region us-east-1 \
    --apns-platform-arn <arn> --gcm-platform-arn <arn>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.AccessKeyID == "" && inputs.SecretAccessKey == "" && inputs.Region == "" &&
				inputs.ApnsArn == "" && inputs.GcmArn == "" {
				return fmt.Errorf(
					"set replaces the entire SNS configuration, so pass at least one of --aws-access-key-id, " +
						"--aws-secret-access-key, --aws-region, --apns-platform-arn or --gcm-platform-arn. " +
						"To change a single field, use 'auth0 guardian factors push update-sns'",
				)
			}

			body := &managementv3.SetGuardianFactorsProviderPushNotificationSnsRequestContent{}
			if inputs.AccessKeyID != "" {
				body.AwsAccessKeyID = &inputs.AccessKeyID
			}
			if inputs.SecretAccessKey != "" {
				body.AwsSecretAccessKey = &inputs.SecretAccessKey
			}
			if inputs.Region != "" {
				body.AwsRegion = &inputs.Region
			}
			if inputs.ApnsArn != "" {
				body.SnsApnsPlatformApplicationArn = &inputs.ApnsArn
			}
			if inputs.GcmArn != "" {
				body.SnsGcmPlatformApplicationArn = &inputs.GcmArn
			}

			var resp *managementv3.SetGuardianFactorsProviderPushNotificationSnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.SetSnsProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set SNS configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push sns configuration updated", guardianSnsRows(
				resp.GetAwsAccessKeyID(),
				resp.GetAwsSecretAccessKey(),
				resp.GetAwsRegion(),
				resp.GetSnsApnsPlatformApplicationArn(),
				resp.GetSnsGcmPlatformApplicationArn(),
			), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianSnsAccessKeyID.RegisterString(cmd, &inputs.AccessKeyID, "")
	guardianSnsSecretAccessKey.RegisterString(cmd, &inputs.SecretAccessKey, "")
	guardianSnsRegion.RegisterString(cmd, &inputs.Region, "")
	guardianSnsApnsArn.RegisterString(cmd, &inputs.ApnsArn, "")
	guardianSnsGcmArn.RegisterString(cmd, &inputs.GcmArn, "")

	return cmd
}

func updateGuardianPushSnsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		AccessKeyID     string
		SecretAccessKey string
		Region          string
		ApnsArn         string
		GcmArn          string
	}

	cmd := &cobra.Command{
		Use:   "update-sns",
		Args:  cobra.NoArgs,
		Short: "Update the SNS configuration",
		Long: "Partially update the Amazon SNS push-notification configuration. Only the fields you provide " +
			"are changed; the rest keep their current values. Run without flags to be prompted for each field, " +
			"pre-filled with the current value (leave the secret access key blank to keep it unchanged).",
		Example: `  auth0 guardian factors push update-sns --aws-region us-west-2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fetch the current configuration so the interactive prompts can
			// default to the existing values (the secret is never pre-filled).
			var current *managementv3.GetGuardianFactorsProviderSnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.GuardianFactorPush.GetSnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read SNS configuration: %w", err)
			}

			currentAccessKeyID := current.GetAwsAccessKeyID()
			currentRegion := current.GetAwsRegion()
			currentApnsArn := current.GetSnsApnsPlatformApplicationArn()
			currentGcmArn := current.GetSnsGcmPlatformApplicationArn()
			if err := guardianSnsAccessKeyID.AskU(cmd, &inputs.AccessKeyID, &currentAccessKeyID); err != nil {
				return err
			}
			if err := guardianSnsSecretAccessKey.AskU(cmd, &inputs.SecretAccessKey, nil); err != nil {
				return err
			}
			if err := guardianSnsRegion.AskU(cmd, &inputs.Region, &currentRegion); err != nil {
				return err
			}
			if err := guardianSnsApnsArn.AskU(cmd, &inputs.ApnsArn, &currentApnsArn); err != nil {
				return err
			}
			if err := guardianSnsGcmArn.AskU(cmd, &inputs.GcmArn, &currentGcmArn); err != nil {
				return err
			}

			body := &managementv3.UpdateGuardianFactorsProviderPushNotificationSnsRequestContent{}
			if inputs.AccessKeyID != "" {
				body.AwsAccessKeyID = &inputs.AccessKeyID
			}
			if inputs.SecretAccessKey != "" {
				body.AwsSecretAccessKey = &inputs.SecretAccessKey
			}
			if inputs.Region != "" {
				body.AwsRegion = &inputs.Region
			}
			if inputs.ApnsArn != "" {
				body.SnsApnsPlatformApplicationArn = &inputs.ApnsArn
			}
			if inputs.GcmArn != "" {
				body.SnsGcmPlatformApplicationArn = &inputs.GcmArn
			}

			if err := ansi.Waiting(func() (err error) {
				_, err = cli.apiv3.GuardianFactorPush.UpdateSnsProvider(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update SNS configuration: %w", err)
			}

			// The PATCH response only echoes the fields that were sent, so
			// re-fetch the full configuration to render the complete state.
			var resp *managementv3.GetGuardianFactorsProviderSnsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorPush.GetSnsProvider(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read SNS configuration: %w", err)
			}

			cli.renderer.GuardianDetail("push sns configuration updated", guardianSnsRows(
				resp.GetAwsAccessKeyID(),
				resp.GetAwsSecretAccessKey(),
				resp.GetAwsRegion(),
				resp.GetSnsApnsPlatformApplicationArn(),
				resp.GetSnsGcmPlatformApplicationArn(),
			), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianSnsAccessKeyID.RegisterString(cmd, &inputs.AccessKeyID, "")
	guardianSnsSecretAccessKey.RegisterString(cmd, &inputs.SecretAccessKey, "")
	guardianSnsRegion.RegisterString(cmd, &inputs.Region, "")
	guardianSnsApnsArn.RegisterString(cmd, &inputs.ApnsArn, "")
	guardianSnsGcmArn.RegisterString(cmd, &inputs.GcmArn, "")

	return cmd
}

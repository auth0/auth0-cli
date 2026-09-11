package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var guardianFactorNameOptions = []string{
	string(managementv3.GuardianFactorNameEnumSms),
	string(managementv3.GuardianFactorNameEnumPushNotification),
	string(managementv3.GuardianFactorNameEnumEmail),
	string(managementv3.GuardianFactorNameEnumOtp),
	string(managementv3.GuardianFactorNameEnumDuo),
	string(managementv3.GuardianFactorNameEnumWebauthnRoaming),
	string(managementv3.GuardianFactorNameEnumWebauthnPlatform),
	string(managementv3.GuardianFactorNameEnumRecoveryCode),
}

var (
	guardianFactorName = Argument{
		Name: "Factor",
		Help: "Name of the factor. One of: sms, push-notification, email, otp, duo, webauthn-roaming, webauthn-platform, recovery-code.",
	}
	guardianFactorEnabled = Flag{
		Name:     "Enabled",
		LongForm: "enabled",
		Help:     "Whether the factor is enabled.",
	}
)

func guardianFactorsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "factors",
		Short: "Manage multi-factor authentication factors",
		Long:  "Manage multi-factor authentication (MFA) factors and their providers.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listGuardianFactorsCmd(cli))
	cmd.AddCommand(setGuardianFactorCmd(cli))
	cmd.AddCommand(guardianFactorPhoneCmd(cli))
	cmd.AddCommand(guardianFactorSmsCmd(cli))
	cmd.AddCommand(guardianFactorPushCmd(cli))
	cmd.AddCommand(guardianFactorDuoCmd(cli))

	return cmd
}

func listGuardianFactorsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Short:   "List multi-factor authentication factors",
		Long:    "List all MFA factors and their enabled/disabled status.",
		Aliases: []string{"ls"},
		Example: `  auth0 guardian factors list
  auth0 guardian factors ls
  auth0 guardian factors list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var factors []*managementv3.GuardianFactor
			if err := ansi.Waiting(func() (err error) {
				factors, err = cli.apiv3.GuardianFactor.List(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to list guardian factors: %w", err)
			}

			cli.renderer.GuardianFactorList(factors)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func setGuardianFactorCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Factor  string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.MaximumNArgs(1),
		Short: "Enable or disable a multi-factor authentication factor",
		Long:  "Enable or disable a single MFA factor.",
		Example: `  auth0 guardian factors set sms --enabled
  auth0 guardian factors set email --enabled=false
  auth0 guardian factors set push-notification --enabled --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := guardianFactorName.Pick(cmd, &inputs.Factor, staticPickerOptions(guardianFactorNameOptions)); err != nil {
					return err
				}
			} else {
				inputs.Factor = args[0]
			}

			factor, err := managementv3.NewGuardianFactorNameEnumFromString(inputs.Factor)
			if err != nil {
				return fmt.Errorf("invalid factor %q: %w", inputs.Factor, err)
			}

			if !guardianFactorEnabled.IsSet(cmd) {
				if !canPrompt(cmd) {
					return fmt.Errorf("--enabled is required when running non-interactively (use --enabled or --enabled=false)")
				}
				if err := guardianFactorEnabled.AskBool(cmd, &inputs.Enabled, nil); err != nil {
					return err
				}
			}

			body := &managementv3.SetGuardianFactorRequestContent{Enabled: inputs.Enabled}

			var result *managementv3.SetGuardianFactorResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.GuardianFactor.Set(cmd.Context(), &factor, body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set guardian factor %q: %w", inputs.Factor, err)
			}

			cli.renderer.GuardianFactorSet(result, inputs.Factor)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	guardianFactorEnabled.RegisterBool(cmd, &inputs.Enabled, false)

	return cmd
}

package cli

import (
	"strings"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var (
	botDetectionLevelPossibleValues = []string{"low", "medium", "high"}
	passwordPolicyPossibleValues    = []string{"never", "when_risky", "always"}
)

var bdFlags = botDetectionFlags{
	BotDetectionLevel: Flag{
		Name:         "Bot Detection Level",
		LongForm:     "bot-detection-level",
		ShortForm:    "l",
		Help:         "The level of bot detection sensitivity. Possible values: " + strings.Join(botDetectionLevelPossibleValues, ", ") + ".",
		AlwaysPrompt: true,
	},
	ChallengePasswordPolicy: Flag{
		Name:     "Challenge Password Policy",
		LongForm: "challenge-password-policy",
		Help: "Determines how often to challenge users with a CAPTCHA for password-based login. Possible values: " +
			strings.Join(passwordPolicyPossibleValues, ", ") + ".",
		AlwaysPrompt: true,
	},
	ChallengePasswordlessPolicy: Flag{
		Name:     "Challenge Passwordless Policy",
		LongForm: "challenge-passwordless-policy",
		Help: "Determines how often to challenge users with a CAPTCHA for passwordless login. Possible values: " +
			strings.Join(passwordPolicyPossibleValues, ", ") + ".",
		AlwaysPrompt: true,
	},
	ChallengePasswordResetPolicy: Flag{
		Name:     "Challenge Password Reset Policy",
		LongForm: "challenge-password-reset-policy",
		Help: "Determines how often to challenge users with a CAPTCHA for password reset. Possible values: " +
			strings.Join(passwordPolicyPossibleValues, ", ") + ".",
		AlwaysPrompt: true,
	},
	AllowList: Flag{
		Name:      "Allow List",
		LongForm:  "allowlist",
		ShortForm: "a",
		Help: "List of comma-separated trusted IP addresses that will not have bot detection enforced against them. " +
			"Supports IPv4, IPv6 and CIDR notations.",
		AlwaysPrompt: true,
	},
	MonitoringModeEnabled: Flag{
		Name:         "Monitoring Mode Enabled",
		LongForm:     "monitoring-mode",
		ShortForm:    "m",
		Help:         "Enable (or disable) monitoring mode. When enabled, logs but does not block.",
		AlwaysPrompt: true,
	},
}

type (
	botDetectionFlags struct {
		BotDetectionLevel            Flag
		ChallengePasswordPolicy      Flag
		ChallengePasswordlessPolicy  Flag
		ChallengePasswordResetPolicy Flag
		AllowList                    Flag
		MonitoringModeEnabled        Flag
	}

	botDetectionInputs struct {
		BotDetectionLevel            string
		ChallengePasswordPolicy      string
		ChallengePasswordlessPolicy  string
		ChallengePasswordResetPolicy string
		AllowList                    []string
		MonitoringModeEnabled        bool
	}
)

func botDetectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bot-detection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"bd"},
		Short:   "Manage bot detection settings",
		Long: "Bot detection protects your applications from automated attacks by detecting and blocking bot traffic. " +
			"Auth0 can challenge suspicious requests with CAPTCHA or block them entirely. " +
			"Configure detection sensitivity, CAPTCHA policies for different authentication flows, and allowlists for trusted IP addresses.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showBotDetectionCmd(cli))
	cmd.AddCommand(updateBotDetectionCmd(cli))

	return cmd
}

func showBotDetectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show bot detection settings",
		Long:  "Display the current bot detection settings.",
		Example: `  auth0 protection bot-detection show
  auth0 ap bd show --json
  auth0 ap bd show --json-compact`,
		RunE: showBotDetectionCmdRun(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func updateBotDetectionCmd(cli *cli) *cobra.Command {
	var inputs botDetectionInputs

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update bot detection settings",
		Long:  "Update the bot detection settings.",
		Example: `  auth0 protection bot-detection update
  auth0 ap bd update --bot-detection-level medium --json-compact
  auth0 ap bd update --bot-detection-level low --challenge-password-policy never
  auth0 ap bd update --monitoring-mode=true --allowlist "198.51.100.42,10.0.0.0/24"
  auth0 ap bd update -l high -a "198.51.100.42" -m=false --json`,
		RunE: updateBotDetectionCmdRun(cli, &inputs),
	}

	bdFlags.BotDetectionLevel.RegisterStringU(cmd, &inputs.BotDetectionLevel, "")
	bdFlags.ChallengePasswordPolicy.RegisterStringU(cmd, &inputs.ChallengePasswordPolicy, "")
	bdFlags.ChallengePasswordlessPolicy.RegisterStringU(cmd, &inputs.ChallengePasswordlessPolicy, "")
	bdFlags.ChallengePasswordResetPolicy.RegisterStringU(cmd, &inputs.ChallengePasswordResetPolicy, "")
	bdFlags.AllowList.RegisterStringSliceU(cmd, &inputs.AllowList, []string{})
	bdFlags.MonitoringModeEnabled.RegisterBoolU(cmd, &inputs.MonitoringModeEnabled, false)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func showBotDetectionCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bd *managementv2.GetBotDetectionSettingsResponseContent
		err := ansi.Waiting(func() (err error) {
			bd, err = cli.apiv2.AttackProtectionBotDetection.Get(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.BotDetectionShow(bd)

		return nil
	}
}

func updateBotDetectionCmdRun(
	cli *cli,
	inputs *botDetectionInputs,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// var bd *management.BotDetection
		// err := ansi.Waiting(func() (err error) {
		// 	bd, err = cli.api.AttackProtection.GetBotDetection(cmd.Context())
		// 	return err
		// })
		// if err != nil {
		// 	return err
		// }

		// if err := bdFlags.BotDetectionLevel.AskU(cmd, &inputs.BotDetectionLevel, bd.BotDetectionLevel); err != nil {
		// 	return err
		// }
		// if inputs.BotDetectionLevel == "" {
		// 	inputs.BotDetectionLevel = bd.GetBotDetectionLevel()
		// }
		// bd.BotDetectionLevel = &inputs.BotDetectionLevel

		// if err := bdFlags.ChallengePasswordPolicy.AskU(cmd, &inputs.ChallengePasswordPolicy, bd.ChallengePasswordPolicy); err != nil {
		// 	return err
		// }
		// if inputs.ChallengePasswordPolicy == "" {
		// 	inputs.ChallengePasswordPolicy = string(bd.GetChallengePasswordPolicy())
		// }
		// bd.ChallengePasswordPolicy = &inputs.ChallengePasswordPolicy

		// if err := bdFlags.ChallengePasswordlessPolicy.AskU(cmd, &inputs.ChallengePasswordlessPolicy, bd.ChallengePasswordlessPolicy); err != nil {
		// 	return err
		// }
		// if inputs.ChallengePasswordlessPolicy == "" {
		// 	inputs.ChallengePasswordlessPolicy = bd.GetChallengePasswordlessPolicy()
		// }
		// bd.ChallengePasswordlessPolicy = &inputs.ChallengePasswordlessPolicy

		// if err := bdFlags.ChallengePasswordResetPolicy.AskU(cmd, &inputs.ChallengePasswordResetPolicy, bd.ChallengePasswordResetPolicy); err != nil {
		// 	return err
		// }
		// if inputs.ChallengePasswordResetPolicy == "" {
		// 	inputs.ChallengePasswordResetPolicy = bd.GetChallengePasswordResetPolicy()
		// }
		// bd.ChallengePasswordResetPolicy = &inputs.ChallengePasswordResetPolicy

		// allowListString := strings.Join(bd.GetAllowList(), ",")
		// if err := bdFlags.AllowList.AskManyU(cmd, &inputs.AllowList, &allowListString); err != nil {
		// 	return err
		// }
		// if len(inputs.AllowList) == 0 {
		// 	inputs.AllowList = bd.GetAllowList()
		// }
		// bd.AllowList = &inputs.AllowList

		// if err := bdFlags.MonitoringModeEnabled.AskBoolU(cmd, &inputs.MonitoringModeEnabled, bd.MonitoringModeEnabled); err != nil {
		// 	return err
		// }
		// bd.MonitoringModeEnabled = &inputs.MonitoringModeEnabled

		// if err := ansi.Waiting(func() error {
		// 	return cli.api.AttackProtection.UpdateBotDetection(cmd.Context(), bd)
		// }); err != nil {
		// 	return err
		// }

		// cli.renderer.BotDetectionUpdate(bd)

		return nil
	}
}

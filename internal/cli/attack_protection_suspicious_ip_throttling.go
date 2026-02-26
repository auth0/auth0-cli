package cli

import (
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var sitFlags = suspiciousIPThrottlingFlags{
	Enabled: Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Enable (or disable) suspicious ip throttling.",
		AlwaysPrompt: true,
	},
	Shields: Flag{
		Name:      "Shields",
		LongForm:  "shields",
		ShortForm: "s",
		Help: "Action to take when a suspicious IP throttling threshold is violated. " +
			"Possible values: block, admin_notification. Comma-separated.",
		AlwaysPrompt: true,
	},
	AllowList: Flag{
		Name:      "Allow List",
		LongForm:  "allowlist",
		ShortForm: "l",
		Help: "List of trusted IP addresses that will not have attack protection enforced against " +
			"them. Comma-separated.",
		AlwaysPrompt: true,
	},
	StagePreLoginMaxAttempts: Flag{
		Name:     "StagePreLoginMaxAttempts",
		LongForm: "pre-login-max",
		Help: "Configuration options that apply before every login attempt. " +
			"Total number of attempts allowed per day.",
		AlwaysPrompt: true,
	},
	StagePreLoginRate: Flag{
		Name:     "StagePreLoginRate",
		LongForm: "pre-login-rate",
		Help: "Configuration options that apply before every login attempt. " +
			"Interval of time, given in milliseconds, at which new attempts are granted.",
		AlwaysPrompt: true,
	},
	StagePreUserRegistrationMaxAttempts: Flag{
		Name:     "StagePreUserRegistrationMaxAttempts",
		LongForm: "pre-registration-max",
		Help: "Configuration options that apply before every user registration attempt. " +
			"Total number of attempts allowed.",
		AlwaysPrompt: true,
	},
	StagePreUserRegistrationRate: Flag{
		Name:     "StagePreUserRegistrationRate",
		LongForm: "pre-registration-rate",
		Help: "Configuration options that apply before every user registration attempt. " +
			"Interval of time, given in milliseconds, at which new attempts are granted.",
		AlwaysPrompt: true,
	},
}

type (
	suspiciousIPThrottlingFlags struct {
		Enabled                             Flag
		Shields                             Flag
		AllowList                           Flag
		StagePreLoginMaxAttempts            Flag
		StagePreLoginRate                   Flag
		StagePreUserRegistrationMaxAttempts Flag
		StagePreUserRegistrationRate        Flag
	}

	suspiciousIPThrottlingInputs struct {
		Enabled                             bool
		Shields                             []string
		AllowList                           []string
		StagePreLoginMaxAttempts            int
		StagePreLoginRate                   int
		StagePreUserRegistrationMaxAttempts int
		StagePreUserRegistrationRate        int
	}
)

func suspiciousIPThrottlingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suspicious-ip-throttling",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"sit"},
		Short:   "Manage suspicious ip throttling settings",
		Long: "Suspicious IP throttling blocks traffic from any IP address that rapidly attempts too many " +
			"logins or signups. This helps protect your applications from high-velocity attacks that target " +
			"multiple accounts. Suspicious IP throttling is enabled by default when you create your Auth0 " +
			"tenant.\n\nWhen Auth0 detects a high number of signup attempts or failed login attempts from an " +
			"IP address, it suspends further attempts from that IP address. You can customize how suspicious " +
			"IP throttling works for your tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showSuspiciousIPThrottlingCmd(cli))
	cmd.AddCommand(updateSuspiciousIPThrottlingCmd(cli))
	cmd.AddCommand(ipsCmd(cli))

	return cmd
}

func showSuspiciousIPThrottlingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show suspicious ip throttling settings",
		Long:  "Display the current suspicious ip throttling settings.",
		Example: `  auth0 protection suspicious-ip-throttling show
  auth0 ap sit show --json`,
		RunE: showSuspiciousIPThrottlingCmdRun(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateSuspiciousIPThrottlingCmd(cli *cli) *cobra.Command {
	var inputs suspiciousIPThrottlingInputs

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update suspicious ip throttling settings",
		Long:  "Update the suspicious ip throttling settings.",
		Example: `  auth0 protection suspicious-ip-throttling update
  auth0 ap sit update --enabled=true
  auth0 ap sit update --enabled=true --allowlist "178.178.178.178"
  auth0 ap sit update --enabled=false --allowlist "178.178.178.178" --shields block
  auth0 ap sit update -e=true -l "178.178.178.178" -s block --json`,
		RunE: updateSuspiciousIPThrottlingCmdRun(cli, &inputs),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	sitFlags.Enabled.RegisterBoolU(cmd, &inputs.Enabled, false)
	sitFlags.Shields.RegisterStringSliceU(cmd, &inputs.Shields, []string{})
	sitFlags.AllowList.RegisterStringSliceU(cmd, &inputs.AllowList, []string{})
	sitFlags.StagePreLoginMaxAttempts.RegisterIntU(cmd, &inputs.StagePreLoginMaxAttempts, 1)
	sitFlags.StagePreLoginRate.RegisterIntU(cmd, &inputs.StagePreLoginRate, 34560)
	sitFlags.StagePreUserRegistrationMaxAttempts.RegisterIntU(cmd, &inputs.StagePreUserRegistrationMaxAttempts, 1)
	sitFlags.StagePreUserRegistrationRate.RegisterIntU(cmd, &inputs.StagePreUserRegistrationRate, 1200)

	return cmd
}

func showSuspiciousIPThrottlingCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var sit *management.SuspiciousIPThrottling
		err := ansi.Waiting(func() (err error) {
			sit, err = cli.api.AttackProtection.GetSuspiciousIPThrottling(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.SuspiciousIPThrottlingShow(sit)

		return nil
	}
}

func updateSuspiciousIPThrottlingCmdRun(
	cli *cli,
	inputs *suspiciousIPThrottlingInputs,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var sit *management.SuspiciousIPThrottling
		err := ansi.Waiting(func() (err error) {
			sit, err = cli.api.AttackProtection.GetSuspiciousIPThrottling(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		if err := sitFlags.Enabled.AskBoolU(cmd, &inputs.Enabled, sit.Enabled); err != nil {
			return err
		}
		if sitFlags.Enabled.IsSet(cmd) || noLocalFlagSet(cmd) {
			sit.Enabled = &inputs.Enabled
		}

		shieldsString := strings.Join(sit.GetShields(), ",")
		if err := sitFlags.Shields.AskManyU(cmd, &inputs.Shields, &shieldsString); err != nil {
			return err
		}
		if len(inputs.Shields) == 0 {
			inputs.Shields = sit.GetShields()
		}
		sit.Shields = &inputs.Shields

		allowListString := strings.Join(sit.GetAllowList(), ",")
		if err := sitFlags.AllowList.AskManyU(
			cmd,
			&inputs.AllowList,
			&allowListString,
		); err != nil {
			return err
		}
		if len(inputs.AllowList) == 0 {
			inputs.AllowList = sit.GetAllowList()
		}
		sit.AllowList = &inputs.AllowList

		// Return early if stage is missing, possible within PSaaS Tenants.
		if sit.Stage == nil {
			cli.renderer.SuspiciousIPThrottlingUpdate(sit)
			return nil
		}

		defaultPreLoginMaxAttempts := strconv.Itoa(sit.Stage.PreLogin.GetMaxAttempts())
		if err := sitFlags.StagePreLoginMaxAttempts.AskIntU(
			cmd,
			&inputs.StagePreLoginMaxAttempts,
			&defaultPreLoginMaxAttempts,
		); err != nil {
			return err
		}
		if inputs.StagePreLoginMaxAttempts == 0 {
			inputs.StagePreLoginMaxAttempts = sit.Stage.PreLogin.GetMaxAttempts()
		}
		sit.Stage.PreLogin.MaxAttempts = &inputs.StagePreLoginMaxAttempts

		defaultPreLoginRate := strconv.Itoa(sit.Stage.PreLogin.GetRate())
		if err := sitFlags.StagePreLoginRate.AskIntU(cmd, &inputs.StagePreLoginRate, &defaultPreLoginRate); err != nil {
			return err
		}
		if inputs.StagePreLoginRate == 0 {
			inputs.StagePreLoginRate = sit.Stage.PreLogin.GetRate()
		}
		sit.Stage.PreLogin.Rate = &inputs.StagePreLoginRate

		defaultPreUserRegistrationMaxAttempts := strconv.Itoa(sit.Stage.PreUserRegistration.GetMaxAttempts())
		if err := sitFlags.StagePreUserRegistrationMaxAttempts.AskIntU(
			cmd,
			&inputs.StagePreUserRegistrationMaxAttempts,
			&defaultPreUserRegistrationMaxAttempts,
		); err != nil {
			return err
		}
		if inputs.StagePreUserRegistrationMaxAttempts == 0 {
			inputs.StagePreUserRegistrationMaxAttempts = sit.Stage.PreUserRegistration.GetMaxAttempts()
		}
		sit.Stage.PreUserRegistration.MaxAttempts = &inputs.StagePreUserRegistrationMaxAttempts

		defaultPreUserRegistrationRate := strconv.Itoa(sit.Stage.PreUserRegistration.GetRate())
		if err := sitFlags.StagePreUserRegistrationRate.AskIntU(
			cmd,
			&inputs.StagePreUserRegistrationRate,
			&defaultPreUserRegistrationRate,
		); err != nil {
			return err
		}
		if inputs.StagePreUserRegistrationRate == 0 {
			inputs.StagePreUserRegistrationRate = sit.Stage.PreUserRegistration.GetRate()
		}
		sit.Stage.PreUserRegistration.Rate = &inputs.StagePreUserRegistrationRate

		if err := ansi.Waiting(func() error {
			return cli.api.AttackProtection.UpdateSuspiciousIPThrottling(cmd.Context(), sit)
		}); err != nil {
			return err
		}

		cli.renderer.SuspiciousIPThrottlingUpdate(sit)

		return nil
	}
}

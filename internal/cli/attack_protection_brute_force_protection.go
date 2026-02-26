package cli

import (
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var bfpFlags = bruteForceProtectionFlags{
	Enabled: Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Enable (or disable) brute force protection.",
		AlwaysPrompt: true,
	},
	Shields: Flag{
		Name:      "Shields",
		LongForm:  "shields",
		ShortForm: "s",
		Help: "Action to take when a brute force protection threshold is violated." +
			" Possible values: block, user_notification. Comma-separated.",
		AlwaysPrompt: true,
	},
	AllowList: Flag{
		Name:      "Allow List",
		LongForm:  "allowlist",
		ShortForm: "l",
		Help: "List of trusted IP addresses that will not have " +
			"attack protection enforced against them. Comma-separated.",
		AlwaysPrompt: true,
	},
	Mode: Flag{
		Name:      "Mode",
		LongForm:  "mode",
		ShortForm: "m",
		Help: "Account Lockout: Determines whether or not IP address is used when counting " +
			"failed attempts. Possible values: count_per_identifier_and_ip, count_per_identifier.",
		AlwaysPrompt: true,
	},
	MaxAttempts: Flag{
		Name:         "MaxAttempts",
		LongForm:     "max-attempts",
		ShortForm:    "a",
		Help:         "Maximum number of unsuccessful attempts.",
		AlwaysPrompt: true,
	},
}

type (
	bruteForceProtectionFlags struct {
		Enabled     Flag
		Shields     Flag
		AllowList   Flag
		Mode        Flag
		MaxAttempts Flag
	}

	bruteForceProtectionInputs struct {
		Enabled     bool
		Shields     []string
		AllowList   []string
		Mode        string
		MaxAttempts int
	}
)

func bruteForceProtectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "brute-force-protection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"bfp"},
		Short:   "Manage brute force protection settings",
		Long:    "Brute-force protection safeguards against a single IP address attacking a single user account.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showBruteForceProtectionCmd(cli))
	cmd.AddCommand(updateBruteForceProtectionCmd(cli))

	return cmd
}

func showBruteForceProtectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show brute force protection settings",
		Long:  "Display the current brute force protection settings.",
		Example: `  auth0 protection brute-force-protection show
  auth0 ap bfp show --json`,
		RunE: showBruteForceProtectionCmdRun(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateBruteForceProtectionCmd(cli *cli) *cobra.Command {
	var inputs bruteForceProtectionInputs

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update brute force protection settings",
		Long:  "Update the brute force protection settings.",
		Example: `  auth0 protection brute-force-protection update
  auth0 ap bfp update --enabled=true
  auth0 ap bfp update --enabled=true --allowlist "156.156.156.156,175.175.175.175"
  auth0 ap bfp update --enabled=false --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3
  auth0 ap bfp update --enabled=true --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3 --mode count_per_identifier_and_ip
  auth0 ap bfp update --enabled=false --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3 --mode count_per_identifier_and_ip --shields user_notification 
  auth0 ap bfp update -e=true -l "156.156.156.156,175.175.175.175" -a 3 -m count_per_identifier_and_ip -s user_notification --json`,
		RunE: updateBruteForceDetectionCmdRun(cli, &inputs),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	bfpFlags.Enabled.RegisterBoolU(cmd, &inputs.Enabled, false)
	bfpFlags.Shields.RegisterStringSliceU(cmd, &inputs.Shields, []string{})
	bfpFlags.AllowList.RegisterStringSliceU(cmd, &inputs.AllowList, []string{})
	bfpFlags.Mode.RegisterString(cmd, &inputs.Mode, "")
	bfpFlags.MaxAttempts.RegisterIntU(cmd, &inputs.MaxAttempts, 1)

	return cmd
}

func showBruteForceProtectionCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bfp *management.BruteForceProtection
		err := ansi.Waiting(func() (err error) {
			bfp, err = cli.api.AttackProtection.GetBruteForceProtection(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.BruteForceProtectionShow(bfp)

		return nil
	}
}

func updateBruteForceDetectionCmdRun(
	cli *cli,
	inputs *bruteForceProtectionInputs,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bfp *management.BruteForceProtection
		err := ansi.Waiting(func() (err error) {
			bfp, err = cli.api.AttackProtection.GetBruteForceProtection(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		if err := bfpFlags.Enabled.AskBoolU(cmd, &inputs.Enabled, bfp.Enabled); err != nil {
			return err
		}
		if bfpFlags.Enabled.IsSet(cmd) || noLocalFlagSet(cmd) {
			bfp.Enabled = &inputs.Enabled
		}

		shieldsString := strings.Join(bfp.GetShields(), ",")
		if err := bfpFlags.Shields.AskManyU(cmd, &inputs.Shields, &shieldsString); err != nil {
			return err
		}
		if len(inputs.Shields) == 0 {
			inputs.Shields = bfp.GetShields()
		}
		bfp.Shields = &inputs.Shields

		allowListString := strings.Join(bfp.GetAllowList(), ",")
		if err := bfpFlags.AllowList.AskManyU(
			cmd,
			&inputs.AllowList,
			&allowListString,
		); err != nil {
			return err
		}
		if len(inputs.AllowList) == 0 {
			inputs.AllowList = bfp.GetAllowList()
		}
		bfp.AllowList = &inputs.AllowList

		if err := bfpFlags.Mode.AskU(cmd, &inputs.Mode, bfp.Mode); err != nil {
			return err
		}
		if inputs.Mode == "" {
			inputs.Mode = bfp.GetMode()
		}
		bfp.Mode = &inputs.Mode

		defaultMaxAttempts := strconv.Itoa(bfp.GetMaxAttempts())
		if err := bfpFlags.MaxAttempts.AskIntU(cmd, &inputs.MaxAttempts, &defaultMaxAttempts); err != nil {
			return err
		}
		if inputs.MaxAttempts == 0 {
			inputs.MaxAttempts = bfp.GetMaxAttempts()
		}
		bfp.MaxAttempts = &inputs.MaxAttempts

		if err := ansi.Waiting(func() error {
			return cli.api.AttackProtection.UpdateBruteForceProtection(cmd.Context(), bfp)
		}); err != nil {
			return err
		}

		cli.renderer.BruteForceProtectionUpdate(bfp)

		return nil
	}
}

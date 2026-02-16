package cli

import (
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var bpdFlags = breachedPasswordDetectionFlags{
	Enabled: Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Enable (or disable) breached password detection.",
		AlwaysPrompt: true,
	},
	Shields: Flag{
		Name:         "Shields",
		LongForm:     "shields",
		ShortForm:    "s",
		Help:         "Action to take when a breached password is detected. Possible values: block, user_notification, admin_notification. Comma-separated.",
		AlwaysPrompt: true,
	},
	AdminNotificationFrequency: Flag{
		Name:      "Admin Notification Frequency",
		LongForm:  "admin-notification-frequency",
		ShortForm: "f",
		Help: "When \"admin_notification\" is enabled, determines how often email notifications " +
			"are sent. Possible values: immediately, daily, weekly, monthly. Comma-separated.",
		AlwaysPrompt: true,
	},
	Method: Flag{
		Name:         "Method",
		LongForm:     "method",
		ShortForm:    "m",
		Help:         "The subscription level for breached password detection methods. Use \"enhanced\" to enable Credential Guard. Possible values: standard, enhanced.",
		AlwaysPrompt: true,
	},
}

type (
	breachedPasswordDetectionFlags struct {
		Enabled                    Flag
		Shields                    Flag
		AdminNotificationFrequency Flag
		Method                     Flag
	}

	breachedPasswordDetectionInputs struct {
		Enabled                    bool
		Shields                    []string
		AdminNotificationFrequency []string
		Method                     string
	}
)

func breachedPasswordDetectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "breached-password-detection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"bpd"},
		Short:   "Manage breached password detection settings",
		Long: "Breached password detection protects your applications from bad actors signing up or logging " +
			"in with stolen credentials. Auth0 can notify users and/or block accounts that are at risk.\n\nAuth0 " +
			"tracks large security breaches that occur on major third-party sites. If Auth0 identifies that any of " +
			"your users’ credentials were part of a breach, the breached password detection security feature triggers.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showBreachedPasswordDetectionCmd(cli))
	cmd.AddCommand(updateBreachedPasswordDetectionCmd(cli))

	return cmd
}

func showBreachedPasswordDetectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show breached password detection settings",
		Long:  "Display the current breached password detection settings.",
		Example: `  auth0 protection breached-password-detection show
  auth0 ap bpd show --json`,
		RunE: showBreachedPasswordDetectionCmdRun(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateBreachedPasswordDetectionCmd(cli *cli) *cobra.Command {
	var inputs breachedPasswordDetectionInputs

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update breached password detection settings",
		Long:  "Update the breached password detection settings.",
		Example: `  auth0 protection breached-password-detection update
  auth0 ap bpd update --enabled=true
  auth0 ap bpd update --enabled=true --admin-notification-frequency weekly
  auth0 ap bpd update --enabled=false --admin-notification-frequency weekly --method enhanced
  auth0 ap bpd update --enabled=true --admin-notification-frequency weekly --method enhanced --shields admin_notification
  auth0 ap bpd update -e=true -f weekly -m enhanced -s admin_notification --json`,
		RunE: updateBreachedPasswordDetectionCmdRun(cli, &inputs),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	bpdFlags.Enabled.RegisterBoolU(cmd, &inputs.Enabled, false)
	bpdFlags.Shields.RegisterStringSliceU(cmd, &inputs.Shields, []string{})
	bpdFlags.AdminNotificationFrequency.RegisterStringSliceU(cmd, &inputs.AdminNotificationFrequency, []string{})
	bpdFlags.Method.RegisterString(cmd, &inputs.Method, "")

	return cmd
}

func showBreachedPasswordDetectionCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bpd *management.BreachedPasswordDetection
		err := ansi.Waiting(func() (err error) {
			bpd, err = cli.api.AttackProtection.GetBreachedPasswordDetection(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.BreachedPasswordDetectionShow(bpd)

		return nil
	}
}

func updateBreachedPasswordDetectionCmdRun(
	cli *cli,
	inputs *breachedPasswordDetectionInputs,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bpd *management.BreachedPasswordDetection
		err := ansi.Waiting(func() (err error) {
			bpd, err = cli.api.AttackProtection.GetBreachedPasswordDetection(cmd.Context())
			return err
		})
		if err != nil {
			return err
		}

		if !bpdFlags.Enabled.IsSet(cmd) {
			inputs.Enabled = bpd.GetEnabled()
		}
		if err := bpdFlags.Enabled.AskBoolU(cmd, &inputs.Enabled, bpd.Enabled); err != nil {
			return err
		}
		bpd.Enabled = &inputs.Enabled

		shieldsString := strings.Join(bpd.GetShields(), ",")
		if err := bpdFlags.Shields.AskManyU(cmd, &inputs.Shields, &shieldsString); err != nil {
			return err
		}
		if len(inputs.Shields) == 0 {
			inputs.Shields = bpd.GetShields()
		}
		bpd.Shields = &inputs.Shields

		adminNotificationFrequencyString := strings.Join(bpd.GetAdminNotificationFrequency(), ",")
		if err := bpdFlags.AdminNotificationFrequency.AskManyU(
			cmd,
			&inputs.AdminNotificationFrequency,
			&adminNotificationFrequencyString,
		); err != nil {
			return err
		}
		if len(inputs.AdminNotificationFrequency) == 0 {
			inputs.AdminNotificationFrequency = bpd.GetAdminNotificationFrequency()
		}
		bpd.AdminNotificationFrequency = &inputs.AdminNotificationFrequency

		if err := bpdFlags.Method.AskU(cmd, &inputs.Method, bpd.Method); err != nil {
			return err
		}
		if inputs.Method == "" {
			inputs.Method = bpd.GetMethod()
		}
		bpd.Method = &inputs.Method

		if err := ansi.Waiting(func() error {
			return cli.api.AttackProtection.UpdateBreachedPasswordDetection(cmd.Context(), bpd)
		}); err != nil {
			return err
		}

		cli.renderer.BreachedPasswordDetectionUpdate(bpd)

		return nil
	}
}

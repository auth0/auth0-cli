package cli

import (
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func breachedPasswordDetectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "breached-password-detection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"bpd"},
		Short:   "Manage breached password detection settings",
		Long:    "Manage breached password detection settings.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showBreachedPasswordDetectionCmd(cli))

	return cmd
}

func showBreachedPasswordDetectionCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Show breached password detection settings",
		Long:    "Show breached password detection settings.",
		Example: `auth0 protection breached-password-detection show`,
		RunE:    showBreachedPasswordDetectionCmdRun(cli),
	}
}

func showBreachedPasswordDetectionCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bpd *management.BreachedPasswordDetection
		err := ansi.Waiting(func() (err error) {
			bpd, err = cli.api.AttackProtection.GetBreachedPasswordDetection()
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.BreachedPasswordDetectionShow(bpd)

		return nil
	}
}

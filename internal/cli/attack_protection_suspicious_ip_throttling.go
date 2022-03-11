package cli

import (
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func suspiciousIPThrottlingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suspicious-ip-throttling",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"sit"},
		Short:   "Manage suspicious ip throttling settings",
		Long:    "Manage suspicious ip throttling settings.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showSuspiciousIPThrottlingCmd(cli))

	return cmd
}

func showSuspiciousIPThrottlingCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Show suspicious ip throttling settings",
		Long:    "Show suspicious ip throttling settings.",
		Example: `auth0 protection suspicious-ip-throttling show`,
		RunE:    showSuspiciousIPThrottlingCmdRun(cli),
	}
}

func showSuspiciousIPThrottlingCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var sit *management.SuspiciousIPThrottling
		err := ansi.Waiting(func() (err error) {
			sit, err = cli.api.AttackProtection.GetSuspiciousIPThrottling()
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.SuspiciousIPThrottlingShow(sit)

		return nil
	}
}

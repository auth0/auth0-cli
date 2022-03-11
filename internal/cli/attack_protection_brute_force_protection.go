package cli

import (
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func bruteForceProtectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "brute-force-protection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"bfp"},
		Short:   "Manage brute force protection settings",
		Long:    "Manage brute force protection settings.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showBruteForceProtectionCmd(cli))

	return cmd
}

func showBruteForceProtectionCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Show brute force protection settings",
		Long:    "Show brute force protection settings.",
		Example: `auth0 protection brute-force-protection show`,
		RunE:    showBruteForceProtectionCmdRun(cli),
	}
}

func showBruteForceProtectionCmdRun(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bfp *management.BruteForceProtection
		err := ansi.Waiting(func() (err error) {
			bfp, err = cli.api.AttackProtection.GetBruteForceProtection()
			return err
		})
		if err != nil {
			return err
		}

		cli.renderer.BruteForceProtectionShow(bfp)

		return nil
	}
}

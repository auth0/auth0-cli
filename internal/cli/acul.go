package cli

import "github.com/spf13/cobra"

func aculCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acul",
		Short: "Advance Customize the Universal Login experience",
		Long:  `Customize the Universal Login experience. This requires a custom domain to be configured for the tenant.`,
	}

	cmd.AddCommand(aculConfigureCmd(cli))
	// Check out the ./acul_scaffolding_app.MD file for more information on the commands below.
	cmd.AddCommand(aculInitCmd1(cli))
	cmd.AddCommand(aculInitCmd2(cli))
	cmd.AddCommand(aculAddScreenCmd(cli))

	return cmd
}

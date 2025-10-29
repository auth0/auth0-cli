package cli

import "github.com/spf13/cobra"

func aculCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acul",
		Short: "Advance Customize the Universal Login experience",
		Long:  `Customize the Universal Login experience. This requires a custom domain to be configured for the tenant.`,
	}

	cmd.AddCommand(aculConfigureCmd(cli))
	cmd.AddCommand(aculInitCmd(cli))
	cmd.AddCommand(aculScreenCmd(cli))
	cmd.AddCommand(aculDevCmd(cli))

	return cmd
}

func aculScreenCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Manage individual screens for Advanced Customizations for Universal Login.",
		Long:  "Manage individual screens for Auth0 Universal Login using ACUL (Advanced Customizations).",
	}

	cmd.AddCommand(aculScreenAddCmd(cli))

	return cmd
}

func aculConfigureCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Advanced Customizations for Universal Login screens.",
		Long:  "Manage screen-level configuration for Auth0 Universal Login using ACUL (Advanced Customizations).",
	}

	cmd.AddCommand(aculConfigGenerateCmd(cli))
	cmd.AddCommand(aculConfigGetCmd(cli))
	cmd.AddCommand(aculConfigSetCmd(cli))
	cmd.AddCommand(aculConfigListCmd(cli))
	cmd.AddCommand(aculConfigDocsCmd(cli))

	return cmd
}

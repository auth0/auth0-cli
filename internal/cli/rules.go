package cli

import (
	"github.com/spf13/cobra"
)

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "manage rules for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	return cmd
}

func listRulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your rules",
		Long:  `Lists the rules in your current tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rules, err := cli.api.Client.Rule.List()

			if err != nil {
				return err
			}

			cli.renderer.RulesList(rules)
			return nil
		},
	}

	return cmd
}

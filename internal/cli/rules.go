package cli

import (
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var name, script string
var order int
var enabled bool

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "manage rules for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(createRulesCmd(cli))

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

func createRulesCmd(cli *cli) *cobra.Command {
	var flags struct {
		name    string
		script  string
		order   int
		enabled bool
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new rule",
		Long:  "Create a new rule in your current tenant.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &management.Rule{
				Name:    &flags.name,
				Script:  &flags.script,
				Order:   &flags.order,
				Enabled: &flags.enabled,
			}

			err := cli.api.Client.Rule.Create(r)

			if err != nil {
				return err
			}

			cli.renderer.Infof("Your rule `%s` was successfully created.", flags.name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name of this rule (required)")
	cmd.Flags().StringVarP(&script, "script", "s", "", "Code to be executed when this rule runs (required)")
	cmd.Flags().IntVarP(&order, "order", "o", 0, "Order that this rule should execute in relative to other rules. Lower-valued rules execute first.")
	cmd.Flags().BoolVarP(&enabled, "enabled", "e", false, "Whether the rule is enabled (true), or disabled (false).")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("script")

	return cmd
}

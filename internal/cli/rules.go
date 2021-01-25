package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var rules []string

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "manage rules for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(enableRuleCmd(cli))
	cmd.AddCommand(disableRuleCmd(cli))
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

func enableRuleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "enable rule",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(rules) == 0 {
				return errors.New("No rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Got following rules:\n%s\n", rules)

			enable := true
			err := cli.api.Client.Rule.Update(rules[0], &management.Rule{Enabled: &enable})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&rules, "rules", "r", nil, "rule ids")
	cmd.MarkFlagRequired("rules")

	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "disable rule",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("in prerun, %d", len(rules))
			if len(rules) == 0 {
				fmt.Print("rules empty")
				return errors.New("no rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&rules, "rules", "r", nil, "rule ids")

	return cmd
}

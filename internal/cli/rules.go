package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

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
			rules, err := getRules(cli)

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
	var name string
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "enable rule(s)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return errors.New("No rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := getRules(cli)
			if err != nil {
				return err
			}

			rule := findRuleByName(name, data.Rules)
			if rule != nil {
				err := enableRule(rule, cli)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("No rule found with name: \"%s\"", name)
			}

			// @TODO Only display modified rules
			rules, err := getRules(cli)

			if err != nil {
				return err
			}

			cli.renderer.RulesList(rules)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "rule name")
	cmd.MarkPersistentFlagRequired("name")
	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "disable rule",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return errors.New("No rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := getRules(cli)
			if err != nil {
				return err
			}

			rule := findRuleByName(name, data.Rules)
			if rule != nil {
				err := disableRule(rule, cli)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("No rule found with name: \"%s\"", name)
			}

			// @TODO Only display modified rules
			rules, err := getRules(cli)

			if err != nil {
				return err
			}

			cli.renderer.RulesList(rules)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "rule name")
	cmd.MarkPersistentFlagRequired("name")

	return cmd
}

// @TODO move to rules package
func getRules(cli *cli) (list *management.RuleList, err error) {
	return cli.api.Client.Rule.List()
}

func findRuleByName(name string, rules []*management.Rule) *management.Rule {
	var foundRule *management.Rule
	for _, aRule := range rules {
		if (*aRule.Name) == name {
			foundRule = aRule
			break
		}
	}
	return foundRule
}

func enableRule(rule *management.Rule, cli *cli) error {
	return cli.api.Client.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(true)})
}

func disableRule(rule *management.Rule, cli *cli) error {
	return cli.api.Client.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(false)})
}

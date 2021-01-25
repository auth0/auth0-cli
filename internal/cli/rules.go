package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

var name string

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

			ruleExists, ruleIdx := findRuleByName(name, data.Rules)
			if ruleExists {
				err := enableRule(data.Rules[ruleIdx], cli)
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
	// @TODO Take a look at this later
	// cmd.MarkFlagRequired("name")

	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
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

			ruleExists, ruleIdx := findRuleByName(name, data.Rules)
			if ruleExists {
				err := disableRule(data.Rules[ruleIdx], cli)
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
	// @TODO Take a look at this later
	// cmd.MarkFlagRequired("name")

	return cmd
}

// @TODO move to rules package
func getRules(cli *cli) (list *management.RuleList, err error) {
	list, err = cli.api.Client.Rule.List()
	return
}

func findRuleByName(name string, rules []*management.Rule) (exists bool, idx int) {
	exists = false
	for i, aRule := range rules {
		if (*aRule.Name) == name {
			exists = true
			idx = i
			break
		}
	}
	return
}

func enableRule(rule *management.Rule, cli *cli) (err error) {
	err = cli.api.Client.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(true)})
	return
}

func disableRule(rule *management.Rule, cli *cli) (err error) {
	err = cli.api.Client.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(false)})
	return
}

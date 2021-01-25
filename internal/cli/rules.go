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
		Short: "enable rule(s)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(rules) == 0 {
				return errors.New("No rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// fmt.Printf("Got following rules (%d):\n%s\n", len(rules), rules)

			// @TODO Cleanup error handling, some can pass some can fail
			updateErrors := enableRules(rules, cli.api.Client.Rule)
			if updateErrors != nil {
				for _, err := range updateErrors {
					fmt.Println(err)
				}
				return errors.New("Some rule updates failed")
			}

			// @TODO Only display modified rules
			rules, err := cli.api.Client.Rule.List()

			if err != nil {
				return err
			}

			cli.renderer.RulesList(rules)

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&rules, "rules", "r", nil, "rule ids")
	// @TODO Take a look at this later
	// err := cmd.MarkFlagRequired("rules")

	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "disable rule(s)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(rules) == 0 {
				return errors.New("No rules to process")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// fmt.Printf("Got following rules (%d):\n%s\n", len(rules), rules)

			// @TODO Cleanup error handling, some can pass some can fail
			updateErrors := disableRules(rules, cli.api.Client.Rule)
			if updateErrors != nil {
				for _, err := range updateErrors {
					fmt.Println(err)
				}
				return errors.New("Some rule updates failed")
			}

			// @TODO Only display modified rules
			rules, err := cli.api.Client.Rule.List()

			if err != nil {
				return err
			}

			cli.renderer.RulesList(rules)

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&rules, "rules", "r", nil, "rule ids")
	// @TODO Take a look at this later
	// err := cmd.MarkFlagRequired("rules")

	return cmd
}

// @TODO refactor to rules package
// @TODO can probably run these concurrently

func enableRules(ruleIds []string, ruleManager *management.RuleManager) []error {
	var updateErrors []error
	enable := true
	for _, ruleID := range ruleIds {
		err := ruleManager.Update(ruleID, &management.Rule{Enabled: &enable})
		if err != nil {
			updateErrors = append(updateErrors, err)
		}
	}

	if len(updateErrors) != 0 {
		return updateErrors
	}

	return nil
}

func disableRules(ruleIds []string, ruleManager *management.RuleManager) []error {
	var updateErrors []error
	enable := false
	for _, ruleID := range ruleIds {
		err := ruleManager.Update(ruleID, &management.Rule{Enabled: &enable})
		if err != nil {
			updateErrors = append(updateErrors, err)
		}
	}

	if len(updateErrors) != 0 {
		return updateErrors
	}

	return nil
}

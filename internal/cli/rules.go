package cli

import (
	"fmt"
	"regexp"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	ruleID      = "id"
	ruleName    = "name"
	ruleScript  = "script"
	ruleOrder   = "order"
	ruleEnabled = "enabled"
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
	cmd.AddCommand(createRulesCmd(cli))
	cmd.AddCommand(deleteRulesCmd(cli))
	cmd.AddCommand(updateRulesCmd(cli))

	return cmd
}

func listRulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your rules",
		Long:  `Lists the rules in your current tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rules *management.RuleList
			err := ansi.Spinner("Loading rules", func() error {
				var err error
				rules, err = getRules(cli)
				return err
			})

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
	var flags struct {
		Name string
	}

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a rule",
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, ruleName) {
				input := prompt.TextInput(
					ruleName, "Name:",
					"Name of the rule.",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			err := ansi.Spinner("Enabling rule", func() error {
				var err error
				data, err := getRules(cli)
				if err != nil {
					return err
				}

				rule := findRuleByName(flags.Name, data.Rules)
				if rule != nil {
					err := enableRule(rule, cli)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("No rule found with name: %q", flags.Name)
				}
				return nil
			})

			if err != nil {
				return err
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

	cmd.Flags().StringVarP(&flags.Name, ruleName, "n", "", "Name of the rule.")
	mustRequireFlags(cmd, ruleName)

	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name string
	}

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a rule",
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, ruleName) {
				input := prompt.TextInput(
					ruleName, "Name:",
					"Name of the rule.",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			err := ansi.Spinner("Disabling rule", func() error {
				var err error
				data, err := getRules(cli)
				if err != nil {
					return err
				}

				rule := findRuleByName(flags.Name, data.Rules)
				if rule != nil {
					if err := disableRule(rule, cli); err != nil {
						return err
					}
				} else {
					return fmt.Errorf("No rule found with name: %q", flags.Name)
				}
				return nil
			})

			if err != nil {
				return err
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

	cmd.Flags().StringVarP(&flags.Name, ruleName, "n", "", "rule name")
	mustRequireFlags(cmd, ruleName)

	return cmd
}

func createRulesCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name    string
		Script  string
		Order   int
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new rule",
		Long: `Create a new rule:

    auth0 rules create --name "My Rule" --script "function (user, context, callback) { console.log( 'Hello, world!' ); return callback(null, user, context); }"
		`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, ruleName) {
				input := prompt.TextInput(
					"name", "Name:",
					"Name of the rule. You can change the rule name later in the rule settings.",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleScript) {
				input := prompt.TextInput(ruleScript, "Script:", "Script of the rule.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleOrder) {
				input := prompt.TextInputDefault(ruleOrder, "Order:", "Order of the rule.", "0", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleEnabled) {
				input := prompt.BoolInput(ruleEnabled, "Enabled:", "Enable the rule.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			r := &management.Rule{
				Name:    &flags.Name,
				Script:  &flags.Script,
				Order:   &flags.Order,
				Enabled: &flags.Enabled,
			}

			err := ansi.Spinner("Creating rule", func() error {
				return cli.api.Rule.Create(r)
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Your rule `%s` was successfully created.", flags.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Name, ruleName, "n", "", "Name of this rule (required)")
	cmd.Flags().StringVarP(&flags.Script, ruleScript, "s", "", "Code to be executed when this rule runs (required)")
	cmd.Flags().IntVarP(&flags.Order, ruleOrder, "o", 0, "Order that this rule should execute in relative to other rules. Lower-valued rules execute first.")
	cmd.Flags().BoolVarP(&flags.Enabled, ruleEnabled, "e", false, "Whether the rule is enabled (true), or disabled (false).")
	mustRequireFlags(cmd, ruleName, ruleScript)

	return cmd
}

func deleteRulesCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID   string
		Name string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a rule",
		Long: `Delete a rule:

	auth0 rules delete --id "12345"`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if flags.ID != "" && flags.Name != "" {
				return fmt.Errorf("TMI! 🤯 use either --name or --id")
			}

			prepareInteractivity(cmd)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, ruleID) {
				input := prompt.TextInput(ruleID, "Id:", "Id of the rule.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			var r *management.Rule
			ruleIDPattern := "^rul_[A-Za-z0-9]{16}$"
			re := regexp.MustCompile(ruleIDPattern)

			if flags.ID != "" {
				if !re.Match([]byte(flags.ID)) {
					return fmt.Errorf("Rule with id %q does not match pattern %s", flags.ID, ruleIDPattern)
				}

				rule, err := cli.api.Rule.Read(flags.ID)
				if err != nil {
					return err
				}
				r = rule
			} else {
				data, err := getRules(cli)
				if err != nil {
					return err
				}
				if rule := findRuleByName(flags.Name, data.Rules); rule != nil {
					r = rule
				} else {
					return fmt.Errorf("No rule found with name: %q", flags.Name)
				}
			}

			err := ansi.Spinner("Deleting rule", func() error {
				return cli.api.Rule.Delete(*r.ID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, ruleID, "i", "", "ID of the rule to delete (required)")
	cmd.Flags().StringVarP(&flags.Name, ruleName, "n", "", "Name of the rule to delete")

	return cmd
}

func updateRulesCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID      string
		Name    string
		Script  string
		Order   int
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "update a rule",
		Long: `Update a rule:

    auth0 rules update --id "12345" --name "My Updated Rule" --script "function (user, context, callback) { console.log( 'Hello, world!' ); return callback(null, user, context); }" --order 1 --enabled true
		`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, ruleID) {
				input := prompt.TextInput(ruleID, "Id:", "Id of the rule.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleName) {
				input := prompt.TextInput(
					"name", "Name:",
					"Name of the rule. You can change the rule name later in the rule settings.",
					"",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleScript) {
				input := prompt.TextInput(ruleScript, "Script:", "Script of the rule.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleOrder) {
				input := prompt.TextInput(ruleOrder, "Order:", "Order of the rule.", "0", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, ruleEnabled) {
				input := prompt.BoolInput(ruleEnabled, "Enabled:", "Enable the rule.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			r := &management.Rule{
				Name:    &flags.Name,
				Script:  &flags.Script,
				Order:   &flags.Order,
				Enabled: &flags.Enabled,
			}

			err := ansi.Spinner("Updating rule", func() error {
				return cli.api.Rule.Update(flags.ID, r)
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Your rule `%s` was successfully updated.", flags.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, ruleID, "i", "", "ID of the rule to update (required)")
	cmd.Flags().StringVarP(&flags.Name, ruleName, "n", "", "Name of this rule")
	cmd.Flags().StringVarP(&flags.Script, ruleScript, "s", "", "Code to be executed when this rule runs")
	cmd.Flags().IntVarP(&flags.Order, ruleOrder, "o", 0, "Order that this rule should execute in relative to other rules. Lower-valued rules execute first.")
	cmd.Flags().BoolVarP(&flags.Enabled, ruleEnabled, "e", false, "Whether the rule is enabled (true), or disabled (false).")
	mustRequireFlags(cmd, ruleID)

	return cmd
}

// @TODO move to rules package
func getRules(cli *cli) (list *management.RuleList, err error) {
	return cli.api.Rule.List()
}

func findRuleByName(name string, rules []*management.Rule) *management.Rule {
	for _, r := range rules {
		if auth0.StringValue(r.Name) == name {
			return r
		}
	}
	return nil
}

func enableRule(rule *management.Rule, cli *cli) error {
	return cli.api.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(true)})
}

func disableRule(rule *management.Rule, cli *cli) error {
	return cli.api.Rule.Update(rule.GetID(), &management.Rule{Enabled: auth0.Bool(false)})
}

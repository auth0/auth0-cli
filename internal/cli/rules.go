package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	ruleID     = "id"
	ruleScript = "script"
)

var (
	ruleNameRequired = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the rule.",
		IsRequired: true,
	}

	ruleName = Flag{
		Name:      "Name",
		LongForm:  "name",
		ShortForm: "n",
		Help:      "Name of the rule.",
	}

	ruleTemplate = Flag{
		Name:      "Template",
		LongForm:  "template",
		ShortForm: "t",
		Help:      "Template to use for the rule.",
	}

	ruleTemplateOptions = flagOptionsFromMapping(ruleTemplateMappings)

	ruleEnabled = Flag{
		Name:      "Enabled",
		LongForm:  "enabled",
		ShortForm: "e",
		Help:      "Enable (or disable) a rule.",
	}

	ruleTemplateMappings = map[string]string{
		"Empty Rule": ruleTemplateEmptyRule,
	}
)

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage rules for clients",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(createRuleCmd(cli))
	cmd.AddCommand(deleteRuleCmd(cli))
	cmd.AddCommand(updateRuleCmd(cli))

	return cmd
}

func listRulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your rules",
		Long:  `List the rules in your current tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rules []*management.Rule
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

/*
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
				input := prompt.TextInput(ruleName, "Name:", "Name of the rule.", true)

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
*/

/*
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
				input := prompt.TextInput(ruleName, "Name:", "Name of the rule.", true)

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
*/

func createRuleCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name     string
		Template string
		Enabled  bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new rule",
		Long: `Create a new rule:

auth0 rules create --name "My Rule" --template [empty-rule]"
		`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ruleNameRequired.Ask(cmd, &flags.Name); err != nil {
				return err
			}

			if err := ruleTemplate.Select(cmd, &flags.Template, ruleTemplateOptions); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			script, err := prompt.CaptureInputViaEditor(
				ruleTemplateMappings[flags.Template],
				flags.Name+".*.js",
			)
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			rule := &management.Rule{
				Name:    &flags.Name,
				Script:  auth0.String(script),
				Enabled: &flags.Enabled,
			}

			err = ansi.Spinner("Creating rule", func() error {
				return cli.api.Rule.Create(rule)
			})

			if err != nil {
				return fmt.Errorf("Unable to create rule: %w", err)
			}

			cli.renderer.RulesCreate(rule)
			return nil
		},
	}

	ruleNameRequired.RegisterString(cmd, &flags.Name, "")
	ruleTemplate.RegisterString(cmd, &flags.Template, "")
	ruleEnabled.RegisterBool(cmd, &flags.Enabled, true)

	return cmd
}

func deleteRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a rule",
		Long: `Delete a rule:

auth0 rules delete rul_d2VSaGlyaW5n`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				var err error
				inputs.ID, err = promptForRuleViaDropdown(cli, cmd)
				if err != nil {
					return err
				}

				if inputs.ID == "" {
					cli.renderer.Infof("There are currently no rules.")
					return nil
				}
			}

			err := ansi.Spinner("Deleting rule", func() error {
				return cli.api.Rule.Delete(inputs.ID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func updateRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	var flags struct {
		Name    string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "update a rule",
		Long: `Update a rule:

auth0 rules update --id  rul_d2VSaGlyaW5n --name "My Updated Rule" --enabled=false
		`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				var err error
				inputs.ID, err = promptForRuleViaDropdown(cli, cmd)
				if err != nil {
					return err
				}

				if inputs.ID == "" {
					cli.renderer.Infof("There are currently no rules.")
					return nil
				}
			}

			if err := ruleName.AskU(cmd, &flags.Name); err != nil {
				return err
			}

			var rule *management.Rule
			err := ansi.Spinner("Fetching rule", func() error {
				var err error
				rule, err = cli.api.Rule.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("Failed to fetch rule with ID: %s %v", inputs.ID, err)
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			script, err := prompt.CaptureInputViaEditor(
				rule.GetScript(),
				rule.GetName()+".*.js",
			)
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			// Since name is optional, no need to specify what they chose.
			if flags.Name == "" {
				flags.Name = rule.GetName()
			}

			err = ansi.Spinner("Updating rule", func() error {
				return cli.api.Rule.Update(inputs.ID, &management.Rule{
					Name:    &flags.Name,
					Script:  &script,
					Enabled: &flags.Enabled,
				})
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Your rule `%s` was successfully updated.", flags.Name)
			return nil
		},
	}

	ruleName.RegisterStringU(cmd, &flags.Name, "")
	ruleEnabled.RegisterBool(cmd, &flags.Enabled, true)

	return cmd
}

// @TODO move to rules package
func getRules(cli *cli) ([]*management.Rule, error) {
	list, err := cli.api.Rule.List()
	if err != nil {
		return nil, err
	}
	return list.Rules, nil
}

func promptForRuleViaDropdown(cli *cli, cmd *cobra.Command) (id string, err error) {
	dropdown := Flag{Name: "Rule"}

	var rules []*management.Rule

	// == Start experimental dropdown for names => id.
	//    TODO(cyx): Consider extracting this
	//    pattern once we've done more of it.
	err = ansi.Spinner("Fetching your rules", func() error {
		rules, err = getRules(cli)
		return err
	})

	if err != nil || len(rules) == 0 {
		return "", err
	}

	mapping := map[string]string{}
	for _, r := range rules {
		mapping[r.GetName()] = r.GetID()
	}

	var name string
	if err := dropdown.Select(cmd, &name, flagOptionsFromMapping(mapping)); err != nil {
		return "", err
	}

	return mapping[name], nil
}

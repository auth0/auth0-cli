package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	ruleName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the rule.",
		IsRequired: true,
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
		"Empty rule":                ruleTemplateEmptyRule,
		"Add email to access token": ruleTemplateAddEmailToAccessToken,
		"Check last password reset": ruleTemplateCheckLastPasswordReset,
		"IP address allow list":     ruleTemplateIPAddressAllowList,
		"IP address deny list":      ruleTemplateIPAddressDenyList,
	}
)

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage resources for rules",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(createRuleCmd(cli))
	cmd.AddCommand(showRuleCmd(cli))
	cmd.AddCommand(updateRuleCmd(cli))
	cmd.AddCommand(deleteRuleCmd(cli))

	return cmd
}

func listRulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your rules",
		Long:  `List the rules in your current tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rules []*management.Rule
			err := ansi.Waiting(func() error {
				ruleList, err := cli.api.Rule.List()
				if err != nil {
					return err
				}
				rules = ruleList.Rules
				return nil
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

func createRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
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
			if err := ruleName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := ruleTemplate.Select(cmd, &inputs.Template, ruleTemplateOptions, nil); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			script, err := prompt.CaptureInputViaEditor(
				ruleTemplateMappings[inputs.Template],
				inputs.Name+".*.js",
			)
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			rule := &management.Rule{
				Name:    &inputs.Name,
				Script:  auth0.String(script),
				Enabled: &inputs.Enabled,
			}

			err = ansi.Waiting(func() error {
				return cli.api.Rule.Create(rule)
			})

			if err != nil {
				return fmt.Errorf("Unable to create rule: %w", err)
			}

			cli.renderer.RuleCreate(rule)
			return nil
		},
	}

	ruleName.RegisterString(cmd, &inputs.Name, "")
	ruleTemplate.RegisterString(cmd, &inputs.Template, "")
	ruleEnabled.RegisterBool(cmd, &inputs.Enabled, true)

	return cmd
}

func showRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a rule",
		Long: `Show a rule:

auth0 rules show <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				// TODO(cyx): Consider making a primitive for
				// Argument to ask using a provided func.
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

			var rule *management.Rule

			err := ansi.Waiting(func() error {
				var err error
				rule, err = cli.api.Rule.Read(inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load rule. The ID %v specified doesn't exist", inputs.ID)
			}

			cli.renderer.RuleShow(rule)
			return nil
		},
	}

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
				// TODO(cyx): Consider making a primitive for
				// Argument to ask using a provided func.
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
		ID      string
		Name    string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a rule",
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
				// TODO(cyx): Consider making a primitive for
				// Argument to ask using a provided func.
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

			var rule *management.Rule
			err := ansi.Waiting(func() error {
				var err error
				rule, err = cli.api.Rule.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("Failed to fetch rule with ID: %s %v", inputs.ID, err)
			}

			if err := ruleName.AskU(cmd, &inputs.Name, rule.Name); err != nil {
				return err
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
			if inputs.Name == "" {
				inputs.Name = rule.GetName()
			}

			// Prepare rule payload for update. This will also be
			// re-hydrated by the SDK, which we'll use below during
			// display.
			rule = &management.Rule{
				Name:    &inputs.Name,
				Script:  &script,
				Enabled: &inputs.Enabled,
			}

			err = ansi.Waiting(func() error {
				return cli.api.Rule.Update(inputs.ID, rule)
			})

			if err != nil {
				return err
			}

			cli.renderer.RuleUpdate(rule)
			return nil
		},
	}

	ruleName.RegisterStringU(cmd, &inputs.Name, "")
	ruleEnabled.RegisterBool(cmd, &inputs.Enabled, true)

	return cmd
}

func promptForRuleViaDropdown(cli *cli, cmd *cobra.Command) (id string, err error) {
	dropdown := Flag{Name: "Rule"}

	var rules []*management.Rule

	// == Start experimental dropdown for names => id.
	//    TODO(cyx): Consider extracting this
	//    pattern once we've done more of it.
	err = ansi.Waiting(func() error {
		list, err := cli.api.Rule.List()
		if err != nil {
			return err
		}
		rules = list.Rules
		return nil
	})

	if err != nil || len(rules) == 0 {
		return "", err
	}

	mapping := map[string]string{}
	for _, r := range rules {
		mapping[r.GetName()] = r.GetID()
	}

	var name string
	if err := dropdown.Select(cmd, &name, flagOptionsFromMapping(mapping), nil); err != nil {
		return "", err
	}

	return mapping[name], nil
}

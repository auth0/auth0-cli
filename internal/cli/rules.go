package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	ruleID = Argument{
		Name: "Rule ID",
		Help: "Id of the rule.",
	}

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

	ruleEnabled = Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Enable (or disable) a rule.",
		AlwaysPrompt: true,
	}

	ruleScript = Flag{
		Name:       "Script",
		LongForm:   "script",
		ShortForm:  "s",
		Help:       "Script contents for the rule.",
		IsRequired: true,
	}

	ruleTemplateOptions = pickerOptions{
		{"Empty rule", ruleTemplateEmptyRule},
		{"Add email to access token", ruleTemplateAddEmailToAccessToken},
		{"Check last password reset", ruleTemplateCheckLastPasswordReset},
		{"Simple domain allow list", ruleTemplateSimpleDomainAllowList},
		{"IP address allow list", ruleTemplateIPAddressAllowList},
		{"IP address deny list", ruleTemplateIPAddressDenyList},
	}
)

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage resources for rules",
		Long:  "Manage resources for rules.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(createRuleCmd(cli))
	cmd.AddCommand(showRuleCmd(cli))
	cmd.AddCommand(updateRuleCmd(cli))
	cmd.AddCommand(deleteRuleCmd(cli))
	cmd.AddCommand(enableRuleCmd(cli))
	cmd.AddCommand(disableRuleCmd(cli))

	return cmd
}

func listRulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your rules",
		Long: `List your existing rules. To create one try:
auth0 rules create`,
		Example: `auth0 rules list
auth0 rules ls`,
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
		Script   string
		Enabled  bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new rule",
		Long:  "Create a new rule.",
		Example: `auth0 rules create
auth0 rules create --name "My Rule"
auth0 rules create -n "My Rule" --template "Empty rule"
auth0 rules create -n "My Rule" -t "Empty rule" --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ruleName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := ruleTemplate.Select(cmd, &inputs.Template, ruleTemplateOptions.labels(), nil); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			err := ruleScript.EditorPrompt(
				cmd,
				&inputs.Script,
				ruleTemplateOptions.getValue(inputs.Template),
				inputs.Name+".*.js",
				cli.ruleEditorHint,
			)
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			rule := &management.Rule{
				Name:    &inputs.Name,
				Script:  auth0.String(inputs.Script),
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
		Long:  "Show a rule.",
		Example: `auth0 rules show 
auth0 rules show <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions)
				if err != nil {
					return err
				}
			}

			var rule *management.Rule

			err := ansi.Waiting(func() error {
				var err error
				rule, err = cli.api.Rule.Read(inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load rule: %w", err)
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
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete a rule",
		Long:  "Delete a rule.",
		Example: `auth0 rules delete 
auth0 rules delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions)
				if err != nil {
					return err
				}
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting Rule", func() error {
				_, err := cli.api.Rule.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete rule: %w", err)
				}

				return cli.api.Rule.Delete(inputs.ID)
			})
		},
	}

	return cmd
}

func updateRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID      string
		Name    string
		Script  string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a rule",
		Long:  "Update a rule.",
		Example: `auth0 rules update <id> 
auth0 rules update <id> --name "My Updated Rule"
auth0 rules update <id> -n "My Updated Rule" --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions)
				if err != nil {
					return err
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

			if !ruleEnabled.IsSet(cmd) {
				inputs.Enabled = auth0.BoolValue(rule.Enabled)
			}

			if err := ruleEnabled.AskBoolU(cmd, &inputs.Enabled, rule.Enabled); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			err = ruleScript.EditorPromptU(
				cmd,
				&inputs.Script,
				rule.GetScript(),
				rule.GetName()+".*.js",
				cli.ruleEditorHint,
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
				Script:  &inputs.Script,
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

func enableRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID      string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "enable",
		Args:  cobra.MaximumNArgs(1),
		Short: "Enable a rule",
		Long:  "Enable a rule.",
		Example: `auth0 rules enable <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions)
				if err != nil {
					return err
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

			rule = &management.Rule{
				Enabled: auth0.Bool(true),
			}

			err = ansi.Waiting(func() error {
				return cli.api.Rule.Update(inputs.ID, rule)
			})

			if err != nil {
				return err
			}

			cli.renderer.RuleEnable(rule)
			return nil
		},
	}

	return cmd
}

func disableRuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID      string
		Enabled bool
	}

	cmd := &cobra.Command{
		Use:   "disable",
		Args:  cobra.MaximumNArgs(1),
		Short: "Disable a rule",
		Long:  "Disable a rule.",
		Example: `auth0 rules disable <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions)
				if err != nil {
					return err
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

			rule = &management.Rule{
				Enabled: auth0.Bool(false),
			}

			err = ansi.Waiting(func() error {
				return cli.api.Rule.Update(inputs.ID, rule)
			})

			if err != nil {
				return err
			}

			cli.renderer.RuleDisable(rule)
			return nil
		},
	}

	return cmd
}

func (c *cli) rulePickerOptions() (pickerOptions, error) {
	list, err := c.api.Rule.List()
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Rules {
		opts = append(opts, pickerOption{value: r.GetID(), label: r.GetName()})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no rules.")
	}

	return opts, nil
}

func (c *cli) ruleEditorHint() {
	c.renderer.Infof("%s once you close the editor, the rule will be saved. To cancel, CTRL+C.", ansi.Faint("Hint:"))
}

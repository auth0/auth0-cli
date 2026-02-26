package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
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

var rulesDeprecationText = "Rules are deprecated and will be removed in the near future. Users should migrate all rules to actions. See https://auth0.com/docs/customize/actions/migrate/migrate-from-rules-to-actions for more details.\n\n"
var rulesDeprecationLogText = ansi.Red(ansi.Bold("DEPRECATED! ")) + ansi.Red(rulesDeprecationText)
var rulesDeprecationDocumentationText = "*DEPRECATED!* " + rulesDeprecationText

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage resources for rules",
		Long: rulesDeprecationDocumentationText + "Rules can be used in a variety of situations as part of the authentication pipeline where " +
			"protocol-specific artifacts are generated.",
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
		Long:    rulesDeprecationDocumentationText + "List your existing rules. To create one, run: `auth0 rules create`.",
		Example: `  auth0 rules list
  auth0 rules ls
  auth0 rules ls --json
  auth0 rules ls --json-compact
  auth0 rules ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rules []*management.Rule

			err := ansi.Waiting(func() error {
				ruleList, err := cli.api.Rule.List(cmd.Context())
				if err != nil {
					return err
				}

				rules = ruleList.Rules

				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to list rules: %w", err)
			}

			cli.renderer.Warnf(rulesDeprecationLogText)
			cli.renderer.RulesList(rules)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

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
		Long: rulesDeprecationDocumentationText + "Create a new rule.\n\n" +
			"To create interactively, use `auth0 rules create` with no arguments.\n\n" +
			"To create non-interactively, supply the name, template and other information through the flags.",
		Example: `  auth0 rules create
  auth0 rules create --enabled=true
  auth0 rules create --enabled=true --name "My Rule" 
  auth0 rules create --enabled=false --name "My Rule" --template "Empty rule"
  auth0 rules create --enabled=true --name "My Rule" --template "Empty rule" --script "$(cat path/to/script.js)"
  auth0 rules create -e=true -n "My Rule" -t "Empty rule" -s "$(cat path/to/script.js)" --json
  auth0 rules create -e=true -n "My Rule" -t "Empty rule" -s "$(cat path/to/script.js)" --json-compact
  echo "{\"name\":\"piping-name\",\"script\":\"console.log('test')\"}" | auth0 rules create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rule := &management.Rule{}
			pipedInput := iostream.PipedInput()

			if len(pipedInput) > 0 {
				err := json.Unmarshal(pipedInput, rule)
				if err != nil {
					return fmt.Errorf("failed to unmarshal JSON input: %w", err)
				}
			} else {
				if err := ruleName.Ask(cmd, &inputs.Name, nil); err != nil {
					return err
				}

				if err := ruleTemplate.Select(cmd, &inputs.Template, ruleTemplateOptions.labels(), nil); err != nil {
					return err
				}

				err := ruleScript.OpenEditor(
					cmd,
					&inputs.Script,
					ruleTemplateOptions.getValue(inputs.Template),
					inputs.Name+".*.js",
					cli.ruleEditorHint,
				)
				if err != nil {
					return fmt.Errorf("failed to capture input from the editor: %w", err)
				}

				rule = &management.Rule{
					Name:    &inputs.Name,
					Script:  auth0.String(inputs.Script),
					Enabled: &inputs.Enabled,
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Rule.Create(cmd.Context(), rule)
			}); err != nil {
				return fmt.Errorf("failed to create rule: %w", err)
			}

			cli.renderer.Warnf(rulesDeprecationLogText)
			cli.renderer.RuleCreate(rule)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	ruleName.RegisterString(cmd, &inputs.Name, "")
	ruleTemplate.RegisterString(cmd, &inputs.Template, "")
	ruleEnabled.RegisterBool(cmd, &inputs.Enabled, true)
	ruleScript.RegisterString(cmd, &inputs.Script, "")

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
		Long:  rulesDeprecationDocumentationText + "Display information about a rule.",
		Example: `  auth0 rules show 
  auth0 rules show <rule-id>
  auth0 rules show <rule-id> --json
  auth0 rules show <rule-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions); err != nil {
					return err
				}
			}

			var rule *management.Rule

			err := ansi.Waiting(func() error {
				var err error
				rule, err = cli.api.Rule.Read(cmd.Context(), inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("failed to read rule with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.Warnf(rulesDeprecationLogText)
			cli.renderer.RuleShow(rule)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteRuleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a rule",
		Long: rulesDeprecationDocumentationText + "Delete a rule.\n\n" +
			"To delete interactively, use `auth0 rules delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the rule id and the `--force` flag to skip confirmation.",
		Example: `  auth0 rules delete 
  auth0 rules rm
  auth0 rules delete <rule-id>
  auth0 rules delete <rule-id> --force
  auth0 rules delete <rule-id> <rule-id2> <rule-idn>
  auth0 rules delete <rule-id> <rule-id2> <rule-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := ruleID.PickMany(cmd, &ids, cli.rulePickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting Rule(s)", ids, func(_ int, id string) error {
				if id != "" {
					if _, err := cli.api.Rule.Read(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete rule with ID %q: %w", id, err)
					}

					if err := cli.api.Rule.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete rule with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

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
		Long: rulesDeprecationDocumentationText + "Update a rule.\n\n" +
			"To update interactively, use `auth0 rules update` with no arguments.\n\n" +
			"To update non-interactively, supply the rule id and other information through the flags.",
		Example: `  auth0 rules update <id>
  auth0 rules update <rule-id> --enabled=true
  auth0 rules update <rule-id> --enabled=false --name "My Updated Rule"
  auth0 rules update <rule-id> --enabled=true --name "My Updated Rule" --script "$(cat path/to/script.js)"
  auth0 rules update <rule-id> -e=true -n "My Updated Rule" -s "$(cat path/to/script.js)" --json
  auth0 rules update <rule-id> -e=true -n "My Updated Rule" -s "$(cat path/to/script.js)" --json-compact
  echo "{\"id\":\"rul_ks3dUazcU3b6PqkH\",\"name\":\"piping-name\"}" | auth0 rules update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			updatedRule := &management.Rule{}
			pipedInput := iostream.PipedInput()
			if len(pipedInput) > 0 {
				if err := json.Unmarshal(pipedInput, updatedRule); err != nil {
					return fmt.Errorf("invalid JSON input: %w", err)
				}

				inputs.ID = updatedRule.GetID()
				updatedRule.ID = nil
			} else {
				if len(args) > 0 {
					inputs.ID = args[0]
				} else {
					if err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions); err != nil {
						return err
					}
				}

				var oldRule *management.Rule
				err := ansi.Waiting(func() (err error) {
					oldRule, err = cli.api.Rule.Read(cmd.Context(), inputs.ID)
					return err
				})
				if err != nil {
					return fmt.Errorf("failed to fetch rule with ID %q: %w", inputs.ID, err)
				}

				if err := ruleName.AskU(cmd, &inputs.Name, oldRule.Name); err != nil {
					return err
				}
				if err := ruleEnabled.AskBoolU(cmd, &inputs.Enabled, oldRule.Enabled); err != nil {
					return err
				}

				err = ruleScript.OpenEditorU(
					cmd,
					&inputs.Script,
					oldRule.GetScript(),
					oldRule.GetName()+".*.js",
				)
				if err != nil {
					return fmt.Errorf("failed to capture input from the editor: %w", err)
				}

				if !cli.force && canPrompt(cmd) {
					var confirmed bool
					if err := prompt.AskBool("Do you want to save the rule script?", &confirmed, true); err != nil {
						return fmt.Errorf("failed to capture prompt input: %w", err)
					}
					if !confirmed {
						return nil
					}
				}

				updatedRule.Enabled = &inputs.Enabled
				if inputs.Name != "" {
					updatedRule.Name = &inputs.Name
				}
				if inputs.Script != "" {
					updatedRule.Script = &inputs.Script
				}
			}

			err := ansi.Waiting(func() error {
				return cli.api.Rule.Update(cmd.Context(), inputs.ID, updatedRule)
			})
			if err != nil {
				return fmt.Errorf("failed to update rule with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.Warnf(rulesDeprecationLogText)
			cli.renderer.RuleUpdate(updatedRule)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	ruleName.RegisterStringU(cmd, &inputs.Name, "")
	ruleEnabled.RegisterBool(cmd, &inputs.Enabled, true)
	ruleScript.RegisterStringU(cmd, &inputs.Script, "")

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
		Long:  rulesDeprecationDocumentationText + "Enable a rule.",
		Example: `  auth0 rules enable
  auth0 rules enable <rule-id>
  auth0 rules enable <rule-id> --json
  auth0 rules enable <rule-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions); err != nil {
					return err
				}
			}

			var rule *management.Rule
			err := ansi.Waiting(func() error {
				var err error
				rule, err = cli.api.Rule.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to fetch rule with ID %q: %w", inputs.ID, err)
			}

			rule = &management.Rule{
				Enabled: auth0.Bool(true),
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Rule.Update(cmd.Context(), inputs.ID, rule)
			}); err != nil {
				return fmt.Errorf("failed to update rule with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.Warnf(rulesDeprecationLogText)
			cli.renderer.RuleEnable(rule)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

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
		Long:  rulesDeprecationDocumentationText + "Disable a rule.",
		Example: `  auth0 rules disable
  auth0 rules disable <rule-id>
  auth0 rules disable <rule-id> --json
  auth0 rules disable <rule-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := ruleID.Pick(cmd, &inputs.ID, cli.rulePickerOptions); err != nil {
					return err
				}
			}

			var rule *management.Rule
			err := ansi.Waiting(func() (err error) {
				rule, err = cli.api.Rule.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to fetch rule with ID %q: %w", inputs.ID, err)
			}

			rule = &management.Rule{
				Enabled: auth0.Bool(false),
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Rule.Update(cmd.Context(), inputs.ID, rule)
			}); err != nil {
				return fmt.Errorf("failed to update rule with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RuleDisable(rule)
			cli.renderer.Warnf(rulesDeprecationLogText)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func (c *cli) rulePickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Rule.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Rules {
		opts = append(opts, pickerOption{value: r.GetID(), label: r.GetName()})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no rules to choose from")
	}

	return opts, nil
}

func (c *cli) ruleEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the rule will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}

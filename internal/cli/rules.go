package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func rulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "manage rules for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRulesCmd(cli))
	cmd.AddCommand(createRulesCmd(cli))
	cmd.AddCommand(deleteRulesCmd(cli))

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
		Long: `Create a new rule:

    auth0 rules create --name "My Rule" --script "function (user, context, callback) { console.log( 'Hello, world!' ); return callback(null, user, context); }"
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &management.Rule{
				Name:    &flags.name,
				Script:  &flags.script,
				Order:   &flags.order,
				Enabled: &flags.enabled,
			}

			err := ansi.Spinner("Creating rule", func() error {
				return cli.api.Client.Rule.Create(r)
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Your rule `%s` was successfully created.", flags.name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Name of this rule (required)")
	cmd.Flags().StringVarP(&flags.script, "script", "s", "", "Code to be executed when this rule runs (required)")
	cmd.Flags().IntVarP(&flags.order, "order", "o", 0, "Order that this rule should execute in relative to other rules. Lower-valued rules execute first.")
	cmd.Flags().BoolVarP(&flags.enabled, "enabled", "e", false, "Whether the rule is enabled (true), or disabled (false).")

	return cmd
}

func deleteRulesCmd(cli *cli) *cobra.Command {
	var flags struct {
		id string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a rule",
		Long: `Delete a rule:

	auth0 rules delete --id "12345"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &management.Rule{ID: &flags.id}
			
			// TODO: Should add validation of rule
			// TODO: Would be nice to prompt user confirmation before proceeding with delete

			err := ansi.Spinner("Deleting rule", func() error {
				return cli.api.Client.Rule.Delete(*r.ID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.id, "id", "i", "", "ID of the rule to delete (required)")

	return cmd
}

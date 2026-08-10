package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func organizationsClientGrantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "organizations",
		Short: "Manage organizations of a client grant",
		Long:  "Manage the organizations associated with a client grant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listOrganizationsClientGrantCmd(cli))

	return cmd
}

func listOrganizationsClientGrantCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List the organizations of a client grant",
		Long:    "List the organizations associated with a client grant.",
		Example: `  auth0 client-grants organizations list
  auth0 client-grants organizations ls <client-grant-id>
  auth0 client-grants organizations list <client-grant-id> --number 100
  auth0 client-grants organizations ls <client-grant-id> -n 100 --json
  auth0 client-grants organizations list <client-grant-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := clientGrantID.Pick(cmd, &inputs.ID, cli.clientGrantPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var organizations []*managementv3.Organization
			if err := ansi.Waiting(func() error {
				page, err := cli.apiv3.ClientGrantOrganization.List(cmd.Context(), inputs.ID, &managementv3.ListClientGrantOrganizationsRequestParameters{})
				if err != nil {
					return err
				}

				iter := page.Iterator()
				for iter.Next(cmd.Context()) {
					organizations = append(organizations, iter.Current())
					if len(organizations) >= inputs.Number {
						break
					}
				}
				return iter.Err()
			}); err != nil {
				return fmt.Errorf("failed to list organizations for client grant with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ClientGrantOrganizationList(organizations)

			return nil
		},
	}

	clientGrantNumber.Help = "Number of organizations to retrieve. Minimum 1, maximum 1000."
	clientGrantNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

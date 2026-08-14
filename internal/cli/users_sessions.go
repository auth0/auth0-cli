package cli

import (
	"context"
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var userSessionsNumber = Flag{
	Name:      "Number",
	LongForm:  "number",
	ShortForm: "n",
	Help:      "Number of user sessions to retrieve. Minimum 1, maximum 1000.",
}

func userSessionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage a user's sessions",
		Long:  "Manage the sessions of an existing user. List a user's sessions or delete all of them at once.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserSessionsCmd(cli))
	cmd.AddCommand(deleteUserSessionsCmd(cli))

	return cmd
}

func listUserSessionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List a user's sessions",
		Long:  "List the active sessions of an existing user.",
		Example: `  auth0 users sessions list
  auth0 users sessions list <user-id>
  auth0 users sessions list <user-id> --number 100
  auth0 users sessions list <user-id> -n 100 --json
  auth0 users sessions list <user-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			sessions, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*core.Page[*string, *managementv3.SessionResponseContent, *managementv3.ListUserSessionsPaginatedResponseContent], error) {
					return cli.apiv3.UserSession.List(ctx, inputs.ID, &managementv3.ListUserSessionsRequestParameters{})
				})
			if err != nil {
				return fmt.Errorf("failed to list sessions for user with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.SessionList(sessions)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	userSessionsNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func deleteUserSessionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete all of a user's sessions",
		Long: "Delete all sessions for a user.\n\n" +
			"This deletes every session the user has, not a single session. To delete one session by its id, " +
			"use `auth0 sessions delete`.\n\n" +
			"To delete non-interactively, supply the user id and the `--force` flag to skip confirmation.",
		Example: `  auth0 users sessions delete
  auth0 users sessions rm <user-id>
  auth0 users sessions delete <user-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed? This deletes ALL sessions for the user."); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting user sessions", func() error {
				if err := cli.apiv3.UserSession.Delete(cmd.Context(), inputs.ID); err != nil {
					return fmt.Errorf("failed to delete sessions for user with ID %q: %w", inputs.ID, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

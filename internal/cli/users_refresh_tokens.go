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

var userRefreshTokensNumber = Flag{
	Name:      "Number",
	LongForm:  "number",
	ShortForm: "n",
	Help:      "Number of user refresh tokens to retrieve. Minimum 1, maximum 1000.",
}

func userRefreshTokensCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-tokens",
		Short: "Manage a user's refresh tokens",
		Long:  "Manage the refresh tokens of an existing user. List a user's refresh tokens or delete all of them at once.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserRefreshTokensCmd(cli))
	cmd.AddCommand(deleteUserRefreshTokensCmd(cli))

	return cmd
}

func listUserRefreshTokensCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List a user's refresh tokens",
		Long:  "List the refresh tokens of an existing user.",
		Example: `  auth0 users refresh-tokens list
  auth0 users refresh-tokens list <user-id>
  auth0 users refresh-tokens list <user-id> --number 100
  auth0 users refresh-tokens list <user-id> -n 100 --json
  auth0 users refresh-tokens list <user-id> --csv`,
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

			tokens, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*core.Page[*string, *managementv3.RefreshTokenResponseContent, *managementv3.ListRefreshTokensPaginatedResponseContent], error) {
					return cli.apiv3.UserRefreshToken.List(ctx, inputs.ID, &managementv3.ListRefreshTokensRequestParameters{})
				})
			if err != nil {
				return fmt.Errorf("failed to list refresh tokens for user with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RefreshTokenList(tokens)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	userRefreshTokensNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func deleteUserRefreshTokensCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete all of a user's refresh tokens",
		Long: "Delete all refresh tokens for a user.\n\n" +
			"This deletes every refresh token the user has, not a single token. To delete one token by its id, " +
			"use `auth0 refresh-tokens delete`.\n\n" +
			"To delete non-interactively, supply the user id and the `--force` flag to skip confirmation.",
		Example: `  auth0 users refresh-tokens delete
  auth0 users refresh-tokens rm <user-id>
  auth0 users refresh-tokens delete <user-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed? This deletes ALL refresh tokens for the user."); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting user refresh tokens", func() error {
				if err := cli.apiv3.UserRefreshToken.Delete(cmd.Context(), inputs.ID); err != nil {
					return fmt.Errorf("failed to delete refresh tokens for user with ID %q: %w", inputs.ID, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

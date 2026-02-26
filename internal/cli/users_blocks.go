package cli

import (
	"fmt"
	"net/http"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Manage brute-force protection user blocks",
		Long:  "Manage brute-force protection user blocks.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserBlocksCmd(cli))
	cmd.AddCommand(deleteUserBlocksCmd(cli))

	return cmd
}

func listUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userIdentifier string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List brute-force protection blocks for a given user",
		Long:  "List brute-force protection blocks for a given user by user ID, username, phone number or email.",
		Example: `  auth0 users blocks list <user-id|username|email|phone-number>
  auth0 users blocks list <user-id|username|email|phone-number> --json
  auth0 users blocks list <user-id|username|email|phone-number> --json-compact
  auth0 users blocks list "auth0|61b5b6e90783fa19f7c57dad"
  auth0 users blocks list "frederik@travel0.com"
  auth0 users blocks list <user-id|username|email|phone-number> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userIdentifier.Ask(cmd, &inputs.userIdentifier); err != nil {
					return err
				}
			} else {
				inputs.userIdentifier = args[0]
			}

			var userBlocks []*management.UserBlock
			err := ansi.Waiting(func() (err error) {
				userBlocks, err = cli.api.User.Blocks(cmd.Context(), inputs.userIdentifier)
				if mErr, ok := err.(management.Error); ok && mErr.Status() != http.StatusBadRequest {
					return err
				}

				userBlocks, err = cli.api.User.BlocksByIdentifier(cmd.Context(), inputs.userIdentifier)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to list user blocks for user with ID %q: %w", inputs.userIdentifier, err)
			}

			cli.renderer.UserBlocksList(userBlocks)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func deleteUserBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock",
		Short: "Remove brute-force protection blocks for users",
		Long:  "Remove brute-force protection blocks for users by user ID, username, phone number or email.",
		Example: `  auth0 users blocks unblock <user-id1|username1|email1|phone-number1> <user-id2|username2|email2|phone-number2>
  auth0 users blocks unblock "auth0|61b5b6e90783fa19f7c57dad"
  auth0 users blocks unblock "frederik@travel0.com" "poovam@travel0.com"
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				var id string
				if err := userIdentifier.Ask(cmd, &id); err != nil {
					return err
				}
				ids = append(ids, id)
			} else {
				ids = args
			}

			return ansi.ProgressBar("Unblocking user(s)", ids, func(_ int, id string) error {
				if id != "" {
					err := cli.api.User.Unblock(cmd.Context(), id)
					if mErr, ok := err.(management.Error); ok && mErr.Status() != http.StatusBadRequest {
						return fmt.Errorf("failed to unblock user with identifier %s: %w", id, err)
					}

					err = cli.api.User.UnblockByIdentifier(cmd.Context(), id)
					if err != nil {
						return fmt.Errorf("failed to unblock user with identifier %s: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	return cmd
}

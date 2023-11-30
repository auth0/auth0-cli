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
		Long:  "List brute-force protection blocks for a given user.",
		Example: `  auth0 users blocks list <user-identifier>
  auth0 users blocks list <user-identifier> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userIdentifier); err != nil {
					return err
				}
			} else {
				inputs.userIdentifier = args[0]
			}

			var userBlocks []*management.UserBlock
			err := ansi.Waiting(func() (err error) {
				userBlocks, err = cli.api.User.Blocks(cmd.Context(), inputs.userIdentifier)
				if mErr, ok := err.(management.Error); ok && mErr.Status() != http.StatusBadRequest {
					return nil
				}

				userBlocks, err = cli.api.User.BlocksByIdentifier(cmd.Context(), inputs.userIdentifier)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to list user blocks for user with ID %s: %w", inputs.userIdentifier, err)
			}

			cli.renderer.UserBlocksList(userBlocks)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func deleteUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userIdentifier string
	}

	cmd := &cobra.Command{
		Use:     "unblock",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove brute-force protection blocks for a given user",
		Long:    "Remove brute-force protection blocks for a given user.",
		Example: `  auth0 users blocks unblock <user-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userIdentifier); err != nil {
					return err
				}
			} else {
				inputs.userIdentifier = args[0]
			}

			err := ansi.Spinner("Unblocking user...", func() error {
				err := cli.api.User.Unblock(cmd.Context(), inputs.userIdentifier)
				if mErr, ok := err.(management.Error); ok && mErr.Status() != http.StatusBadRequest {
					return nil
				}

				return cli.api.User.UnblockByIdentifier(cmd.Context(), inputs.userIdentifier)
			})
			if err != nil {
				return fmt.Errorf("failed to unblock user with ID %s: %w", inputs.userIdentifier, err)
			}

			return nil
		},
	}

	return cmd
}

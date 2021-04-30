package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	userID = Argument{
		Name: "User ID",
		Help: "Id of the user.",
	}
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage resources for users",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(userBlocksCmd(cli))
	cmd.AddCommand(deleteUserBlocksCmd(cli))
	return cmd
}

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Manage brute-force protection user blocks.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserBlocksCmd(cli))
	return cmd
}

func listUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userID string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.MaximumNArgs(1),
		Short:   "List brute-force protection blocks for a given user",
		Long:    "List brute-force protection blocks for a given user.",
		Example: "auth0 users blocks list <user-id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userID); err != nil {
					return err
				}
			} else {
				inputs.userID = args[0]
			}

			var userBlocks []*management.UserBlock

			err := ansi.Waiting(func() error {
				var err error
				userBlocks, err = cli.api.User.Blocks(inputs.userID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load user blocks %v, error: %w", inputs.userID, err)
			}

			cli.renderer.UserBlocksList(userBlocks)
			return nil
		},
	}

	return cmd
}

func deleteUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userID string
	}

	cmd := &cobra.Command{
		Use:     "unblock",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove brute-force protection blocks for a given user",
		Long:    "Remove brute-force protection blocks for a given user.",
		Example: "auth0 users unblock <user-id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userID); err != nil {
					return err
				}
			} else {
				inputs.userID = args[0]
			}

			err := ansi.Spinner("Deleting blocks for user...", func() error {
				return cli.api.User.Unblock(inputs.userID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

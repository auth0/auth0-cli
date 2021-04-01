package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage resources for users",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(userBlocksCmd(cli))
	return cmd
}

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Manage brute-force protection user blocks.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserBlocksCmd(cli))
	cmd.AddCommand(deleteUserBlocksCmd(cli))
	return cmd
}

func listUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userID string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List brute-force protection blocks for a given user",
		Long: `List brute-force protection blocks for a given user:

auth0 users blocks list <User ID>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.userID = args[0]
			} else {
				return errors.New("User ID is required.")
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
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete brute-force protection blocks for a given user",
		Long: `Delete brute-force protection blocks for a given user:

auth0 users blocks delete <User ID>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.userID = args[0]
			} else {
				return errors.New("User ID is required.")
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

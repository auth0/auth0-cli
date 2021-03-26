package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	userID = Argument{
		Name: "user_id",
		Help: "user_id of the user.",
	}
)

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-blocks",
		Short: "Manage brute-force protection user blocks.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserBlocksByUserIdCmd(cli))
	cmd.AddCommand(deleteUserBlocksByUserIdCmd(cli))
	return cmd
}

func listUserBlocksByUserIdCmd(cli *cli) *cobra.Command {
	var inputs struct {
		user_id string
	}

	cmd := &cobra.Command{
		Use:   "listByUserId",
		Args:  cobra.MaximumNArgs(1),
		Short: "List user-blocks by user_id",
		Long: `List user-blocks by user_id:

auth0 user-blocks listByUserId <user_id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.user_id = args[0]
			} else {
				return errors.New("user_id is required.")
			}

			var userBlocks []*management.UserBlock

			err := ansi.Waiting(func() error {
				var err error
				userBlocks, err = cli.api.User.Blocks(inputs.user_id)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load user blocks %v, error: %w", inputs.user_id, err)
			}

			cli.renderer.UserBlocksList(userBlocks)
			return nil
		},
	}

	return cmd
}

func deleteUserBlocksByUserIdCmd(cli *cli) *cobra.Command {
	var inputs struct {
		user_id string
	}

	cmd := &cobra.Command{
		Use:   "deleteByUserId",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete all user-blocks by user_id",
		Long: `Delete all user-blocks by user_id:

auth0 user-blocks deleteByUserId <user_id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.user_id = args[0]
			} else {
				return errors.New("user_id is required.")
			}

			err := ansi.Spinner("Deleting blocks for user...", func() error {
				return cli.api.User.Unblock(inputs.user_id)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

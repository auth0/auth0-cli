package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	sessionID = Argument{
		Name: "Id",
		Help: "Id of the session.",
	}
	sessionMetadata = Flag{
		Name:         "Metadata",
		LongForm:     "metadata",
		ShortForm:    "m",
		Help:         "Metadata key/value pairs to set on the session, e.g. --metadata key=value. Repeat the flag or comma-separate pairs for multiple values. Passing no pairs clears the metadata.",
		AlwaysPrompt: false,
	}
)

func sessionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage resources for sessions",
		Long:  "Manage sessions for a tenant. Sessions are keyed by session id; to list the sessions for a user, use `auth0 users sessions list`.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showSessionCmd(cli))
	cmd.AddCommand(updateSessionCmd(cli))
	cmd.AddCommand(deleteSessionCmd(cli))
	cmd.AddCommand(revokeSessionCmd(cli))

	return cmd
}

func showSessionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a session",
		Long:  "Display the device, clients, and expiry information about a session.",
		Example: `  auth0 sessions show
  auth0 sessions show <session-id>
  auth0 sessions show <session-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := sessionID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var session *managementv3.GetSessionResponseContent
			if err := ansi.Waiting(func() (err error) {
				session, err = cli.apiv3.Session.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read session with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.SessionShow(session)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func updateSessionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID       string
		Metadata map[string]string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a session",
		Long: "Update the metadata on a session.\n\n" +
			"Metadata is the only writable field on a session. The pairs you pass replace the existing metadata; " +
			"passing no pairs clears it.",
		Example: `  auth0 sessions update <session-id> --metadata key=value
  auth0 sessions update <session-id> -m key1=value1 -m key2=value2
  auth0 sessions update <session-id> --metadata key=value --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := sessionID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			metadata := stringMapToAny(inputs.Metadata)
			body := &managementv3.UpdateSessionRequestContent{
				SessionMetadata: &metadata,
			}

			var session *managementv3.UpdateSessionResponseContent
			if err := ansi.Waiting(func() (err error) {
				session, err = cli.apiv3.Session.Update(cmd.Context(), inputs.ID, body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update session with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.SessionUpdate(session)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	sessionMetadata.RegisterStringMap(cmd, &inputs.Metadata, nil)

	return cmd
}

func deleteSessionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete a session",
		Long: "Delete a session.\n\n" +
			"To delete interactively, use `auth0 sessions delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the session id and the `--force` flag to skip confirmation.",
		Example: `  auth0 sessions delete
  auth0 sessions rm
  auth0 sessions delete <session-id>
  auth0 sessions delete <session-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var id string
			if len(args) == 0 {
				if err := sessionID.Ask(cmd, &id); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting session", func() error {
				if err := cli.apiv3.Session.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete session with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func revokeSessionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke",
		Args:  cobra.MaximumNArgs(1),
		Short: "Revoke a session",
		Long: "Revoke a session and all of its associated refresh tokens.\n\n" +
			"Unlike `delete`, revoke also invalidates every refresh token tied to the session.\n\n" +
			"To revoke non-interactively, supply the session id and the `--force` flag to skip confirmation.",
		Example: `  auth0 sessions revoke
  auth0 sessions revoke <session-id>
  auth0 sessions revoke <session-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var id string
			if len(args) == 0 {
				if err := sessionID.Ask(cmd, &id); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed? This also revokes the session's refresh tokens."); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Revoking session", func() error {
				if err := cli.apiv3.Session.Revoke(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to revoke session with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

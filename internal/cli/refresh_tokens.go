package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	refreshTokenID = Argument{
		Name: "Id",
		Help: "Id of the refresh token.",
	}
	refreshTokenMetadata = Flag{
		Name:         "Metadata",
		LongForm:     "metadata",
		ShortForm:    "m",
		Help:         "Metadata key/value pairs to set on the refresh token, e.g. --metadata key=value. Repeat the flag or comma-separate pairs for multiple values. Passing no pairs clears the metadata.",
		AlwaysPrompt: false,
	}
	refreshTokenRevokeUserID = Flag{
		Name:     "User Id",
		LongForm: "user-id",
		Help:     "Revoke all refresh tokens for this user, instead of a single token by id.",
	}
	refreshTokenRevokeClientID = Flag{
		Name:     "Client Id",
		LongForm: "client-id",
		Help:     "Narrow a user revocation to a single client. Requires --user-id.",
	}
	refreshTokenRevokeAudience = Flag{
		Name:     "Audience",
		LongForm: "audience",
		Help:     "Narrow a user+client revocation to a single API audience. Requires --user-id and --client-id.",
	}
)

func refreshTokensCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-tokens",
		Short: "Manage resources for refresh tokens",
		Long:  "Manage refresh tokens for a tenant. Refresh tokens are keyed by token id; to list the refresh tokens for a user, use `auth0 users refresh-tokens list`.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showRefreshTokenCmd(cli))
	cmd.AddCommand(updateRefreshTokenCmd(cli))
	cmd.AddCommand(deleteRefreshTokenCmd(cli))
	cmd.AddCommand(revokeRefreshTokenCmd(cli))

	return cmd
}

func showRefreshTokenCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a refresh token",
		Long:  "Display the client, session, device, and expiry information about a refresh token.",
		Example: `  auth0 refresh-tokens show
  auth0 refresh-tokens show <token-id>
  auth0 refresh-tokens show <token-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := refreshTokenID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var token *managementv3.GetRefreshTokenResponseContent
			if err := ansi.Waiting(func() (err error) {
				token, err = cli.apiv3.RefreshToken.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read refresh token with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RefreshTokenShow(token)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func updateRefreshTokenCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID       string
		Metadata map[string]string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a refresh token",
		Long: "Update the metadata on a refresh token.\n\n" +
			"Metadata is the only writable field on a refresh token. The pairs you pass replace the existing " +
			"metadata; passing no pairs clears it.",
		Example: `  auth0 refresh-tokens update <token-id> --metadata key=value
  auth0 refresh-tokens update <token-id> -m key1=value1 -m key2=value2
  auth0 refresh-tokens update <token-id> --metadata key=value --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := refreshTokenID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			metadata := stringMapToAny(inputs.Metadata)
			body := &managementv3.UpdateRefreshTokenRequestContent{
				RefreshTokenMetadata: &metadata,
			}

			var token *managementv3.UpdateRefreshTokenResponseContent
			if err := ansi.Waiting(func() (err error) {
				token, err = cli.apiv3.RefreshToken.Update(cmd.Context(), inputs.ID, body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update refresh token with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RefreshTokenUpdate(token)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	refreshTokenMetadata.RegisterStringMap(cmd, &inputs.Metadata, nil)

	return cmd
}

func deleteRefreshTokenCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete a refresh token",
		Long: "Delete a refresh token.\n\n" +
			"To delete interactively, use `auth0 refresh-tokens delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the token id and the `--force` flag to skip confirmation.",
		Example: `  auth0 refresh-tokens delete
  auth0 refresh-tokens rm
  auth0 refresh-tokens delete <token-id>
  auth0 refresh-tokens delete <token-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var id string
			if len(args) == 0 {
				if err := refreshTokenID.Ask(cmd, &id); err != nil {
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

			return ansi.Spinner("Deleting refresh token", func() error {
				if err := cli.apiv3.RefreshToken.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete refresh token with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func revokeRefreshTokenCmd(cli *cli) *cobra.Command {
	var inputs struct {
		UserID   string
		ClientID string
		Audience string
	}

	cmd := &cobra.Command{
		Use:   "revoke",
		Args:  cobra.MaximumNArgs(1),
		Short: "Revoke refresh tokens",
		Long: "Revoke refresh tokens so they can no longer be exchanged for new access tokens.\n\n" +
			"Pass a token id to revoke a single token. Alternatively, use `--user-id` to revoke all of a " +
			"user's tokens, optionally narrowing to a single client with `--client-id` and a single API with " +
			"`--audience` (`--client-id` requires `--user-id`, and `--audience` requires both).\n\n" +
			"To revoke non-interactively, supply the token id and the `--force` flag to skip confirmation.",
		Example: `  auth0 refresh-tokens revoke
  auth0 refresh-tokens revoke <token-id>
  auth0 refresh-tokens revoke <token-id> --force
  auth0 refresh-tokens revoke --user-id <user-id> --force
  auth0 refresh-tokens revoke --user-id <user-id> --client-id <client-id>
  auth0 refresh-tokens revoke --user-id <user-id> --client-id <client-id> --audience <audience>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.ClientID != "" && inputs.UserID == "" {
				return fmt.Errorf("--client-id requires --user-id")
			}
			if inputs.Audience != "" && (inputs.UserID == "" || inputs.ClientID == "") {
				return fmt.Errorf("--audience requires --user-id and --client-id")
			}

			body := &managementv3.RevokeRefreshTokensRequestContent{}
			var target string

			switch {
			case inputs.UserID != "":
				if len(args) > 0 {
					return fmt.Errorf("pass either a token id or --user-id, not both")
				}
				body.UserID = &inputs.UserID
				target = fmt.Sprintf("all refresh tokens for user %q", inputs.UserID)
				if inputs.ClientID != "" {
					body.ClientID = &inputs.ClientID
					target = fmt.Sprintf("refresh tokens for user %q and client %q", inputs.UserID, inputs.ClientID)
				}
				if inputs.Audience != "" {
					body.Audience = &inputs.Audience
					target = fmt.Sprintf("refresh tokens for user %q, client %q and audience %q", inputs.UserID, inputs.ClientID, inputs.Audience)
				}
			default:
				var id string
				if len(args) == 0 {
					if err := refreshTokenID.Ask(cmd, &id); err != nil {
						return err
					}
				} else {
					id = args[0]
				}
				body.IDs = []string{id}
				target = fmt.Sprintf("refresh token with ID %q", id)
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm(fmt.Sprintf("Are you sure you want to revoke %s?", target)); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Revoking refresh tokens", func() error {
				if err := cli.apiv3.RefreshToken.Revoke(cmd.Context(), body); err != nil {
					return fmt.Errorf("failed to revoke %s: %w", target, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	refreshTokenRevokeUserID.RegisterString(cmd, &inputs.UserID, "")
	refreshTokenRevokeClientID.RegisterString(cmd, &inputs.ClientID, "")
	refreshTokenRevokeAudience.RegisterString(cmd, &inputs.Audience, "")

	return cmd
}

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	tokenExchangeProfileID = Argument{
		Name: "Id",
		Help: "Id of the token exchange profile.",
	}

	tokenExchangeProfileName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the token exchange profile.",
		IsRequired: true,
	}

	tokenExchangeProfileSubjectTokenType = Flag{
		Name:       "Subject Token Type",
		LongForm:   "subject-token-type",
		ShortForm:  "s",
		Help:       "Type of the subject token. Must be a valid URI format (e.g., urn:ietf:params:oauth:token-type:jwt). Cannot use reserved prefixes: http://auth0.com, https://auth0.com, http://okta.com, https://okta.com, urn:ietf, urn:auth0, urn:okta.",
		IsRequired: true,
	}

	tokenExchangeProfileActionID = Flag{
		Name:       "Action ID",
		LongForm:   "action-id",
		ShortForm:  "a",
		Help:       "Identifier of the action.",
		IsRequired: true,
	}

	tokenExchangeProfileType = Flag{
		Name:       "Type",
		LongForm:   "type",
		ShortForm:  "t",
		Help:       "Type of the token exchange profile. Currently only 'custom_authentication' is supported.",
		IsRequired: true,
	}
)

func tokenExchangeCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "token-exchange",
		Aliases: []string{"te"},
		Short:   "Manage token exchange profiles",
		Long:    "Manage token exchange profiles. Token exchange profiles enable secure token exchange flows for authentication and authorization.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listTokenExchangeProfilesCmd(cli))
	cmd.AddCommand(createTokenExchangeProfileCmd(cli))
	cmd.AddCommand(showTokenExchangeProfileCmd(cli))
	cmd.AddCommand(updateTokenExchangeProfileCmd(cli))
	cmd.AddCommand(deleteTokenExchangeProfileCmd(cli))

	return cmd
}

func listTokenExchangeProfilesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your token exchange profiles",
		Long:    "List your existing token exchange profiles. To create one, run: `auth0 token-exchange create`.",
		Example: `  auth0 token-exchange list
  auth0 token-exchange ls
  auth0 token-exchange ls --json
  auth0 token-exchange ls --json-compact
  auth0 token-exchange ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.TokenExchangeProfileList

			if err := ansi.Waiting(func() (err error) {
				list, err = cli.api.TokenExchange.List(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to list token exchange profiles: %w", err)
			}

			cli.renderer.TokenExchangeProfileList(list.TokenExchangeProfiles)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showTokenExchangeProfileCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a token exchange profile",
		Long:  "Display the name, subject token type, action ID, type and other information about a token exchange profile.",
		Example: `  auth0 token-exchange show
  auth0 token-exchange show <profile-id>
  auth0 token-exchange show <profile-id> --json
  auth0 token-exchange show <profile-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := tokenExchangeProfileID.Pick(cmd, &inputs.ID, cli.tokenExchangeProfilePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var profile *management.TokenExchangeProfile

			if err := ansi.Waiting(func() (err error) {
				profile, err = cli.api.TokenExchange.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read token exchange profile with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.TokenExchangeProfileShow(profile)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createTokenExchangeProfileCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name             string
		SubjectTokenType string
		ActionID         string
		Type             string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new token exchange profile",
		Long: "Create a new token exchange profile.\n\n" +
			"To create interactively, use `auth0 token-exchange create` with no flags.\n\n" +
			"To create non-interactively, supply the name, subject token type, action ID, and type through the flags.",
		Example: `  auth0 token-exchange create
  auth0 token-exchange create --name "My Token Exchange Profile"
  auth0 token-exchange create --name "My Token Exchange Profile" --subject-token-type "urn:ietf:params:oauth:token-type:jwt"
  auth0 token-exchange create --name "My Token Exchange Profile" --subject-token-type "urn:ietf:params:oauth:token-type:jwt" --action-id "act_123abc" --type "custom_authentication"
  auth0 token-exchange create -n "My Token Exchange Profile" -s "urn:ietf:params:oauth:token-type:jwt" -a "act_123abc" -t "custom_authentication" --json
  auth0 token-exchange create -n "My Token Exchange Profile" -s "urn:ietf:params:oauth:token-type:jwt" -a "act_123abc" -t "custom_authentication" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := tokenExchangeProfileName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := tokenExchangeProfileSubjectTokenType.Ask(cmd, &inputs.SubjectTokenType, nil); err != nil {
				return err
			}

			// Use action picker to select an action with custom-token-exchange trigger.
			if err := tokenExchangeProfileActionID.Pick(cmd, &inputs.ActionID, cli.customTokenExchangeActionPickerOptions); err != nil {
				return err
			}

			// Select type - currently only custom_authentication is supported.
			if err := tokenExchangeProfileType.Select(cmd, &inputs.Type, []string{"custom_authentication"}, nil); err != nil {
				return err
			}

			profile := &management.TokenExchangeProfile{
				Name:             &inputs.Name,
				SubjectTokenType: &inputs.SubjectTokenType,
				ActionID:         &inputs.ActionID,
				Type:             &inputs.Type,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.TokenExchange.Create(cmd.Context(), profile)
			}); err != nil {
				return fmt.Errorf("failed to create token exchange profile: %w", err)
			}

			cli.renderer.TokenExchangeProfileCreate(profile)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	tokenExchangeProfileName.RegisterString(cmd, &inputs.Name, "")
	tokenExchangeProfileSubjectTokenType.RegisterString(cmd, &inputs.SubjectTokenType, "")
	tokenExchangeProfileActionID.RegisterString(cmd, &inputs.ActionID, "")
	tokenExchangeProfileType.RegisterString(cmd, &inputs.Type, "")

	return cmd
}

func updateTokenExchangeProfileCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID               string
		Name             string
		SubjectTokenType string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a token exchange profile",
		Long: "Update a token exchange profile.\n\n" +
			"To update interactively, use `auth0 token-exchange update` with no arguments.\n\n" +
			"To update non-interactively, supply the profile id, name, and subject token type through the flags.\n\n" +
			"Note: Only name and subject token type can be updated. Action ID and type are immutable after creation.",
		Example: `  auth0 token-exchange update
  auth0 token-exchange update <profile-id>
  auth0 token-exchange update <profile-id> --name "Updated Profile Name"
  auth0 token-exchange update <profile-id> --name "Updated Profile Name" --subject-token-type "urn:ietf:params:oauth:token-type:jwt"
  auth0 token-exchange update <profile-id> -n "Updated Profile Name" -s "urn:ietf:params:oauth:token-type:jwt" --json
  auth0 token-exchange update <profile-id> -n "Updated Profile Name" -s "urn:ietf:params:oauth:token-type:jwt" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := tokenExchangeProfileID.Pick(cmd, &inputs.ID, cli.tokenExchangeProfilePickerOptions); err != nil {
					return err
				}
			}

			var current *management.TokenExchangeProfile
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.api.TokenExchange.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read token exchange profile with ID %q: %w", inputs.ID, err)
			}

			if err := tokenExchangeProfileName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			if err := tokenExchangeProfileSubjectTokenType.AskU(cmd, &inputs.SubjectTokenType, current.SubjectTokenType); err != nil {
				return err
			}

			updatedProfile := &management.TokenExchangeProfile{}

			if inputs.Name != "" {
				updatedProfile.Name = &inputs.Name
			}

			if inputs.SubjectTokenType != "" {
				updatedProfile.SubjectTokenType = &inputs.SubjectTokenType
			}

			if err := ansi.Waiting(func() error {
				return cli.api.TokenExchange.Update(cmd.Context(), inputs.ID, updatedProfile)
			}); err != nil {
				return fmt.Errorf("failed to update token exchange profile with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.TokenExchangeProfileUpdate(updatedProfile)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	tokenExchangeProfileName.RegisterStringU(cmd, &inputs.Name, "")
	tokenExchangeProfileSubjectTokenType.RegisterStringU(cmd, &inputs.SubjectTokenType, "")

	return cmd
}

func deleteTokenExchangeProfileCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a token exchange profile",
		Long: "Delete a token exchange profile.\n\n" +
			"To delete interactively, use `auth0 token-exchange delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the profile id and the `--force` flag to skip confirmation.",
		Example: `  auth0 token-exchange delete
  auth0 token-exchange rm
  auth0 token-exchange delete <profile-id>
  auth0 token-exchange delete <profile-id> --force
  auth0 token-exchange delete <profile-id> <profile-id2> <profile-idn>
  auth0 token-exchange delete <profile-id> <profile-id2> <profile-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := tokenExchangeProfileID.PickMany(cmd, &ids, cli.tokenExchangeProfilePickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting token exchange profile(s)", ids, func(i int, id string) error {
				if id != "" {
					if err := cli.api.TokenExchange.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete token exchange profile with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (c *cli) tokenExchangeProfilePickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.TokenExchange.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, p := range list.TokenExchangeProfiles {
		label := fmt.Sprintf("%s %s", p.GetName(), ansi.Faint("("+p.GetID()+")"))
		opts = append(opts, pickerOption{value: p.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no token exchange profiles to choose from. Create one by running: `auth0 token-exchange create`")
	}

	return opts, nil
}

// customTokenExchangeActionPickerOptions returns actions filtered by custom-token-exchange trigger.
func (c *cli) customTokenExchangeActionPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Action.List(
		ctx,
		management.Parameter("triggerId", "custom-token-exchange"),
	)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, action := range list.Actions {
		value := action.GetID()
		label := fmt.Sprintf("%s %s", action.GetName(), ansi.Faint("("+value+")"))
		opts = append(opts, pickerOption{value: value, label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no actions found with trigger type 'custom-token-exchange'. Please create an action with this trigger first")
	}

	return opts, nil
}

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	phoneProviderTwilio = "twilio"
	phoneProviderCustom = "custom"
)

var (
	PhoneProviderNameOptions = []string{
		phoneProviderTwilio,
		phoneProviderCustom,
	}

	phoneProviderID = Argument{
		Name: "Id",
		Help: "Id of the Phone Provider.",
	}

	phoneProviderName = Flag{
		Name:      "Provider",
		LongForm:  "provider",
		ShortForm: "p",
		Help: fmt.Sprintf("Provider name. Can be '%s', or '%s'",
			phoneProviderTwilio,
			phoneProviderCustom),
		AlwaysPrompt: true,
	}

	phoneProviderCredentials = Flag{
		Name:         "Credentials",
		LongForm:     "credentials",
		ShortForm:    "c",
		Help:         "Credentials for the phone provider, formatted as JSON.",
		AlwaysPrompt: true,
	}

	phoneProviderConfiguration = Flag{
		Name:         "Configuration Settings",
		LongForm:     "configuration settings",
		ShortForm:    "s",
		Help:         "Configuration for the phone provider. formatted as JSON.",
		AlwaysPrompt: true,
	}

	phoneProviderDisabled = Flag{
		Name:         "Disabled",
		LongForm:     "disabled",
		ShortForm:    "d",
		Help:         "Whether the provided is disabled (true) or enabled (false).",
		AlwaysPrompt: true,
	}
)

func phoneProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage phone provider",
		Long:  "Manage custom and twilio phone provider for the tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingPhoneProviderCmd(cli))
	cmd.AddCommand(createBrandingPhoneProviderCmd(cli))
	cmd.AddCommand(updateBrandingPhoneProviderCmd(cli))
	cmd.AddCommand(deleteBrandingPhoneProviderCmd(cli))
	return cmd
}

func showBrandingPhoneProviderCmd(cli *cli) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show the Phone provider",
		Long:  "Display information about the phone provider.",
		Example: `  auth0 phone provider show
  auth0 phone provider show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := phoneProviderID.Pick(cmd, &id, cli.phoneProviderPickerOptions); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			var phoneProvider *management.BrandingPhoneProvider

			if err := ansi.Waiting(func() (err error) {
				phoneProvider, err = cli.api.Branding.ReadPhoneProvider(cmd.Context(), id)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read phone provider with ID %q: %w", id, err)
			}

			return cli.renderer.PhoneProviderShow(phoneProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createBrandingPhoneProviderCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name          string
		credentials   string
		configuration string
		disabled      bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create the phone provider",
		Long: "Create the phone provider.\n\n" +
			"To create interactively, use `auth0 phone provider create` with no arguments.\n\n" +
			"To create non-interactively, supply the provider name and other information " +
			"through the flags.",
		Example: `  auth0 phone provider create
  auth0 phone provider create --json
  auth0 phone provider create --provider twilio --disabled=false --credentials='{ "auth_token":"TheAuthToken" }' --configuration='{ "default_from": "admin@example.com", "sid": "+1234567890", "delivery_methods": ["text", "voice"] }'
  auth0 phone provider create --provider custom --enabled=true --default-from-address="admin@example.com"
  auth0 phone provider create -p twilio -d "false" -c '{ "auth_token":"TheAuthToken" }' -s '{ "default_from": "admin@example.com", "sid": "+1234567890", "delivery_methods": ["text", "voice"] }  `,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := phoneProviderName.Select(cmd, &inputs.name, PhoneProviderNameOptions, nil); err != nil {
				return err
			}

			if err := phoneProviderDisabled.AskBool(cmd, &inputs.disabled, nil); err != nil {
				return err
			}

			var credentials *management.BrandingPhoneProviderCredential

			if inputs.name != phoneProviderCustom {
				if err := phoneProviderCredentials.Ask(cmd, &inputs.credentials, nil); err != nil {
					return err
				}

				if err := json.Unmarshal([]byte(inputs.credentials), &credentials); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.name, err)
				}
			}

			var configuration *management.BrandingPhoneProviderConfiguration

			if err := phoneProviderConfiguration.Ask(cmd, &inputs.configuration, nil); err != nil {
				return err
			}

			if len(inputs.configuration) > 0 {
				if err := json.Unmarshal([]byte(inputs.configuration), &configuration); err != nil {
					return fmt.Errorf("provider: %s configuration invalid JSON: %w", inputs.name, err)
				}
			}

			phoneProvider := &management.BrandingPhoneProvider{
				Name:     &inputs.name,
				Disabled: &inputs.disabled,
			}

			phoneProvider.Credentials = credentials

			if configuration != nil {
				phoneProvider.Configuration = configuration
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Branding.CreatePhoneProvider(cmd.Context(), phoneProvider)
			}); err != nil {
				return fmt.Errorf("failed to create phone provider %s: %w", inputs.name, err)
			}

			return cli.renderer.PhoneProviderCreate(phoneProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	phoneProviderName.RegisterString(cmd, &inputs.name, "")
	phoneProviderCredentials.RegisterString(cmd, &inputs.credentials, "")
	phoneProviderConfiguration.RegisterString(cmd, &inputs.configuration, "")
	phoneProviderDisabled.RegisterBool(cmd, &inputs.disabled, false)

	return cmd
}

func (c *cli) phoneProviderPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Branding.ListPhoneProviders(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Providers {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no phone providers to choose from. Create one by running: `auth0 phone provider create`")
	}

	return opts, nil
}

func updateBrandingPhoneProviderCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id            string
		name          string
		credentials   string
		configuration string
		disabled      bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update the phone provider",
		Long: "Update the phone provider.\n\n" +
			"To update interactively, use `auth0 phone provider update` with no arguments.\n\n" +
			"To update non-interactively, supply the provider name and other information " +
			"through the flags.",
		Example: `  auth0 phone provider update
  auth0 phone provider update --json
  auth0 phone provider update --disabled=false
  auth0 phone provider update --credentials='{ "auth_token":"NewAuthToken" }'
  auth0 phone provider update --configuration='{ "delivery_methods": ["voice"] }'
  auth0 phone provider update --configuration='{ "default_from": admin@example.com }'
  auth0 phone provider update --provider twilio --disabled=false --credentials='{ "auth_token":"NewAuthToken" }' --configuration='{ "default_from": "admin@example.com", "delivery_methods": ["voice", "text"] }'
  auth0 phone provider update --provider custom --disabled=false --configuration='{ "delivery_methods": ["voice", "text"] }"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var currentProvider *management.BrandingPhoneProvider

			if len(args) == 0 {
				if err := phoneProviderID.Pick(cmd, &inputs.id, cli.phoneProviderPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.id = args[0]
			}

			if err := ansi.Waiting(func() (err error) {
				currentProvider, err = cli.api.Branding.ReadPhoneProvider(cmd.Context(), inputs.id)
				return
			}); err != nil {
				return fmt.Errorf("failed to read phone provider: %w", err)
			}

			if err := phoneProviderName.SelectU(cmd, &inputs.name, PhoneProviderNameOptions, currentProvider.Name); err != nil {
				return err
			}

			if err := phoneProviderDisabled.AskBoolU(cmd, &inputs.disabled, currentProvider.Disabled); err != nil {
				return err
			}

			var (
				credentials   *management.BrandingPhoneProviderCredential
				configuration *management.BrandingPhoneProviderConfiguration
			)

			phoneProvider := &management.BrandingPhoneProvider{}

			// Check if we are changing providers.
			if len(inputs.name) > 0 && inputs.name != currentProvider.GetName() {
				// Only set the name if we are changing it.
				phoneProvider.Name = &inputs.name

				if inputs.name != phoneProviderCustom {
					if err := phoneProviderCredentials.Ask(cmd, &inputs.credentials, nil); err != nil {
						return err
					}

					if err := json.Unmarshal([]byte(inputs.credentials), &credentials); err != nil {
						return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.name, err)
					}
				} else {
					if err := phoneProviderCredentials.AskU(cmd, &inputs.credentials, nil); err != nil {
						return err
					}
					if err := json.Unmarshal([]byte(inputs.credentials), &credentials); err != nil {
						return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.name, err)
					}
				}
			}

			if err := phoneProviderConfiguration.AskU(cmd, &inputs.configuration, nil); err != nil {
				return err
			}
			if len(inputs.configuration) > 0 {
				if err := json.Unmarshal([]byte(inputs.configuration), &configuration); err != nil {
					return fmt.Errorf("provider: %s configuration invalid JSON: %w", inputs.name, err)
				}
			}

			// Set the flag if it was supplied or entered by the prompt.
			if canPrompt(cmd) || phoneProviderDisabled.IsSet(cmd) {
				phoneProvider.Disabled = &inputs.disabled
			}

			if credentials != nil {
				phoneProvider.Credentials = credentials
			}

			if configuration != nil {
				phoneProvider.Configuration = configuration
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Branding.UpdatePhoneProvider(cmd.Context(), inputs.id, phoneProvider)
			}); err != nil {
				return fmt.Errorf("failed to update phone provider %s: %w", inputs.name, err)
			}

			return cli.renderer.PhoneProviderUpdate(phoneProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	phoneProviderName.RegisterString(cmd, &inputs.name, "")
	phoneProviderCredentials.RegisterString(cmd, &inputs.credentials, "")
	phoneProviderConfiguration.RegisterString(cmd, &inputs.configuration, "")
	phoneProviderDisabled.RegisterBool(cmd, &inputs.disabled, false)

	return cmd
}

func deleteBrandingPhoneProviderCmd(cli *cli) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.NoArgs,
		Short:   "Delete the phone provider",
		Long: "Delete the phone provider.\n\n" +
			"To delete interactively, use `auth0 phone provider delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the phone provider id and the `--force`" +
			" flag to skip confirmation.",
		Example: `auth0 provider delete
auth0 phone provider rm
auth0 phone provider delete <phone-provider-id> --force
auth0 phone provider delete <phone-provider-id>
auth0 phone provider rm --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			if len(args) == 0 {
				if err := phoneProviderID.Pick(cmd, &id, cli.phoneProviderPickerOptions); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Branding.DeletePhoneProvider(cmd.Context(), id)
			}); err != nil {
				return fmt.Errorf("failed to delete phone provider: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

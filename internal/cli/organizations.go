package cli

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	apiOrganizationColorPrimary        = "primary"
	apiOrganizationColorPageBackground = "page_background"
)

var (
	organizationID = Argument{
		Name: "Id",
		Help: "Id of the organization.",
	}

	organizationName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the organization.",
		IsRequired: true,
	}

	organizationDisplay = Flag{
		Name:         "Display Name",
		LongForm:     "display",
		ShortForm:    "d",
		Help:         "Friendly name of the organization.",
		AlwaysPrompt: true,
	}

	organizationLogo = Flag{
		Name:      "Logo URL",
		LongForm:  "logo",
		ShortForm: "l",
		Help:      "URL of the logo to be displayed on the login page.",
	}

	organizationAccent = Flag{
		Name:      "Accent Color",
		LongForm:  "accent",
		ShortForm: "a",
		Help:      "Accent color used to customize the login pages.",
	}

	organizationBackground = Flag{
		Name:      "Background Color",
		LongForm:  "background",
		ShortForm: "b",
		Help:      "Background color used to customize the login pages.",
	}

	organizationMetadata = Flag{
		Name:      "Metadata",
		LongForm:  "metadata",
		ShortForm: "m",
		Help:      "Metadata associated with the organization (max 255 chars). Maximum of 10 metadata properties allowed.",
	}
)

func organizationsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orgs",
		Aliases: []string{"organizations"},
		Short:   "Manage resources for organizations",
		Long:    "Manage resources for organizations.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listOrganizationsCmd(cli))
	cmd.AddCommand(createOrganizationCmd(cli))
	cmd.AddCommand(showOrganizationCmd(cli))
	cmd.AddCommand(updateOrganizationCmd(cli))
	cmd.AddCommand(deleteOrganizationCmd(cli))
	cmd.AddCommand(openOrganizationCmd(cli))

	return cmd
}

func listOrganizationsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your organizations",
		Long: `List your existing organizations. To create one try:
auth0 orgs create`,
		Example: `auth0 orgs list
auth0 orgs ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.OrganizationList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.Organization.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.OrganizationList(list.Organizations)
			return nil
		},
	}

	return cmd
}

func showOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an organization",
		Long:  "Show an organization.",
		Example: `auth0 orgs show 
auth0 orgs show <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var organization *management.Organization

			if err := ansi.Waiting(func() error {
				var err error
				organization, err = cli.api.Organization.Read(url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get an organization with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.OrganizationShow(organization)
			return nil
		},
	}

	return cmd
}

func createOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name            string
		DisplayName     string
		LogoURL         string
		AccentColor    string
		BackgroundColor string
		Metadata        map[string]string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new organization",
		Long:  "Create a new organization.",
		Example: `auth0 orgs create 
auth0 orgs create --name myorganization
auth0 orgs create --n myorganization --display "My Organization"
auth0 orgs create --n myorganization -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
auth0 orgs create --n myorganization -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := organizationName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := organizationDisplay.Ask(cmd, &inputs.DisplayName, nil); err != nil {
				return err
			}

			o := &management.Organization{
				Name:        &inputs.Name,
				DisplayName: &inputs.DisplayName,
				Metadata:    apiOrganizationMetadataFor(inputs.Metadata),
			}

			isLogoURLSet := len(inputs.LogoURL) > 0
			isAccentColorSet := len(inputs.AccentColor) > 0
			isBackgroundColorSet := len(inputs.BackgroundColor) > 0
			isAnyColorSet := isAccentColorSet || isBackgroundColorSet

			if isLogoURLSet || isAnyColorSet {
				o.Branding = &management.OrganizationBranding{}

				if isLogoURLSet {
					o.Branding.LogoUrl = &inputs.LogoURL
				}

				if isAnyColorSet {
					o.Branding.Colors = map[string]string{}

					if isAccentColorSet {
						o.Branding.Colors[apiOrganizationColorPrimary] = inputs.AccentColor
					}
					
					if isBackgroundColorSet {
						o.Branding.Colors[apiOrganizationColorPageBackground] = inputs.BackgroundColor
					}
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Organization.Create(o)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while attempting to create an organization with name '%s': %w", inputs.Name, err)
			}

			cli.renderer.OrganizationCreate(o)
			return nil
		},
	}

	organizationName.RegisterString(cmd, &inputs.Name, "")
	organizationDisplay.RegisterString(cmd, &inputs.DisplayName, "")
	organizationLogo.RegisterString(cmd, &inputs.LogoURL, "")
	organizationAccent.RegisterString(cmd, &inputs.AccentColor, "")
	organizationBackground.RegisterString(cmd, &inputs.BackgroundColor, "")
	organizationMetadata.RegisterStringMap(cmd, &inputs.Metadata, nil)

	return cmd
}

func updateOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		DisplayName     string
		LogoURL         string
		AccentColor    string
		BackgroundColor string
		Metadata        map[string]string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an organization",
		Long:  "Update an organization.",
		Example: `auth0 orgs update <id> 
auth0 orgs update <id> --display "My Organization"
auth0 orgs update <id> -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
auth0 orgs update <id> -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			}

			var current *management.Organization
			err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.Organization.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("Failed to fetch organization with ID: %s %v", inputs.ID, err)
			}

			if err := organizationDisplay.AskU(cmd, &inputs.DisplayName, current.DisplayName); err != nil {
				return err
			}

			if inputs.DisplayName == "" {
				inputs.DisplayName = current.GetDisplayName()
			}

			// Prepare organization payload for update. This will also be
			// re-hydrated by the SDK, which we'll use below during
			// display.
			o := &management.Organization{
				ID:          current.ID,
				DisplayName: &inputs.DisplayName,
			}

			isLogoURLSet := len(inputs.LogoURL) > 0
			isAccentColorSet := len(inputs.AccentColor) > 0
			isBackgroundColorSet := len(inputs.BackgroundColor) > 0
			currentHasBranding := current.Branding != nil
			currentHasColors := currentHasBranding && current.Branding.Colors != nil
			needToAddColors := isAccentColorSet || isBackgroundColorSet || currentHasColors

			if isLogoURLSet || needToAddColors {
				o.Branding = &management.OrganizationBranding{}

				if isLogoURLSet {
					o.Branding.LogoUrl = &inputs.LogoURL
				} else if currentHasBranding {
					o.Branding.LogoUrl = current.Branding.LogoUrl
				}
	
				if needToAddColors {
					o.Branding.Colors = map[string]string{}

					if isAccentColorSet {
						o.Branding.Colors[apiOrganizationColorPrimary] = inputs.AccentColor
					} else if currentHasColors && len(current.Branding.Colors[apiOrganizationColorPrimary]) > 0 {
						o.Branding.Colors[apiOrganizationColorPrimary] = current.Branding.Colors[apiOrganizationColorPrimary]
					}

					if isBackgroundColorSet {
						o.Branding.Colors[apiOrganizationColorPageBackground] = inputs.BackgroundColor
					} else if currentHasColors && len(current.Branding.Colors[apiOrganizationColorPageBackground]) > 0 {
						o.Branding.Colors[apiOrganizationColorPageBackground] = current.Branding.Colors[apiOrganizationColorPageBackground]
					}
				}
			}

			if len(inputs.Metadata) == 0 {
				o.Metadata = current.Metadata
			} else {
				o.Metadata = apiOrganizationMetadataFor(inputs.Metadata)
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Organization.Update(o)
			}); err != nil {
				return err
			}

			cli.renderer.OrganizationUpdate(o)
			return nil
		},
	}

	organizationDisplay.RegisterStringU(cmd, &inputs.DisplayName, "")
	organizationLogo.RegisterStringU(cmd, &inputs.LogoURL, "")
	organizationAccent.RegisterStringU(cmd, &inputs.AccentColor, "")
	organizationBackground.RegisterStringU(cmd, &inputs.BackgroundColor, "")
	organizationMetadata.RegisterStringMapU(cmd, &inputs.Metadata, nil)

	return cmd
}

func deleteOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete an organization",
		Long:  "Delete an organization.",
		Example: `auth0 orgs delete 
auth0 orgs delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting organization", func() error {
				_, err := cli.api.Organization.Read(url.PathEscape(inputs.ID))

				if err != nil {
					return fmt.Errorf("Unable to delete organization: %w", err)
				}

				return cli.api.Organization.Delete(url.PathEscape(inputs.ID))
			})
		},
	}

	return cmd
}

func openOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open organization settings page in the Auth0 Dashboard",
		Long:    "Open organization settings page in the Auth0 Dashboard.",
		Example: "auth0 orgs open <id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatOrganizationDetailsPath(url.PathEscape(inputs.ID)))
			return nil
		},
	}

	return cmd
}

func (c *cli) organizationPickerOptions() (pickerOptions, error) {
	list, err := c.api.Organization.List()
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Organizations {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no organizations.")
	}

	return opts, nil
}

func formatOrganizationDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("organizations/%s/overview", id)
}

func apiOrganizationMetadataFor(metadata map[string]string) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range metadata {
		key := k
		value := v
		res[key] = value
	}
	return res
}

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	apiOrganizationColorPrimary        = "primary"
	apiOrganizationColorPageBackground = "page_background"
)

var (
	organizationID = Argument{
		Name: "Org ID",
		Help: "ID of the organization.",
	}

	invitationID = Argument{
		Name: "Invitation ID",
		Help: "ID of the invitation.",
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

	roleIdentifier = Flag{
		Name:       "Role",
		LongForm:   "role-id",
		ShortForm:  "r",
		Help:       "Role Identifier.",
		IsRequired: true,
	}

	// Purposefully not setting the Help value on the Flag because overridden where appropriate.
	organizationNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
	}

	inviterName = Flag{
		Name:       "Inviter Name",
		LongForm:   "inviter-name",
		ShortForm:  "n",
		Help:       "Name of the person sending the invitation.",
		IsRequired: true,
	}

	inviteeEmail = Flag{
		Name:       "Invitee Email",
		LongForm:   "invitee-email",
		ShortForm:  "e",
		Help:       "Email address of the person being invited.",
		IsRequired: true,
	}

	clientID = Flag{
		Name:       "Client ID",
		LongForm:   "client-id",
		Help:       "Auth0 client ID. Used to resolve the application's login initiation endpoint.",
		IsRequired: true,
	}

	connectionID = Flag{
		Name:     "Connection ID",
		LongForm: "connection-id",
		Help:     "The id of the connection to force invitee to authenticate with.",
	}

	ttlSeconds = Flag{
		Name:      "TTL Seconds",
		LongForm:  "ttl-sec",
		ShortForm: "t",
		Help:      "Number of seconds for which the invitation is valid before expiration.",
	}

	sendInvitationEmail = Flag{
		Name:      "Send Invitation Email",
		LongForm:  "send-email",
		ShortForm: "s",
		Help:      "Whether to send the invitation email to the invitee.",
	}

	roles = Flag{
		Name:      "Roles",
		LongForm:  "roles",
		ShortForm: "r",
		Help:      "Roles IDs to associate with the user.",
	}

	applicationMetadata = Flag{
		Name:      "App Metadata",
		LongForm:  "app-metadata",
		ShortForm: "a",
		Help:      "Data related to the user that does affect the application's core functionality, formatted as JSON",
	}

	userMetadata = Flag{
		Name:      "User Metadata",
		LongForm:  "user-metadata",
		ShortForm: "u",
		Help:      "Data related to the user that does not affect the application's core functionality, formatted as JSON",
	}
)

func organizationsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orgs",
		Aliases: []string{"organizations"},
		Short:   "Manage resources for organizations",
		Long: "The Auth0 Organizations feature best supports business-to-business (B2B) implementations " +
			"that have applications that end-users access.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listOrganizationsCmd(cli))
	cmd.AddCommand(createOrganizationCmd(cli))
	cmd.AddCommand(showOrganizationCmd(cli))
	cmd.AddCommand(updateOrganizationCmd(cli))
	cmd.AddCommand(deleteOrganizationCmd(cli))
	cmd.AddCommand(openOrganizationCmd(cli))
	cmd.AddCommand(membersOrganizationCmd(cli))
	cmd.AddCommand(rolesOrganizationCmd(cli))
	cmd.AddCommand(invitationsOrganizationCmd(cli))

	return cmd
}

func listOrganizationsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your organizations",
		Long:    "List your existing organizations. To create one, run: `auth0 orgs create`.",
		Example: `  auth0 orgs list
  auth0 orgs ls
  auth0 orgs ls --json
  auth0 orgs ls --json-compact
  auth0 orgs ls --csv
  auth0 orgs ls -n 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					res, err := cli.api.Organization.List(cmd.Context(), opts...)
					if err != nil {
						return nil, false, err
					}

					for _, item := range res.Organizations {
						result = append(result, item)
					}

					return result, res.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("failed to list organizations: %w", err)
			}

			var orgs []*management.Organization
			for _, item := range list {
				orgs = append(orgs, item.(*management.Organization))
			}

			cli.renderer.OrganizationList(orgs)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	organizationNumber.Help = "Number of organizations to retrieve. Minimum 1, maximum 1000."
	organizationNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

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
		Long:  "Display information about an organization.",
		Example: `  auth0 orgs show
  auth0 orgs show <org-id>
  auth0 orgs show <org-id> --json
  auth0 orgs show <org-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var organization *management.Organization

			if err := ansi.Waiting(func() error {
				var err error
				organization, err = cli.api.Organization.Read(cmd.Context(), url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("failed to read organization with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.OrganizationShow(organization)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name            string
		DisplayName     string
		LogoURL         string
		AccentColor     string
		BackgroundColor string
		Metadata        map[string]string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new organization",
		Long: "Create a new organization.\n\n" +
			"To create interactively, use `auth0 orgs create` with no arguments.\n\n" +
			"To create non-interactively, supply the name and other information through the flags.",
		Example: `  auth0 orgs create
  auth0 orgs create --name myorganization
  auth0 orgs create -n myorganization --display "My Organization"
  auth0 orgs create -n myorganization -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
  auth0 orgs create -n myorganization -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := organizationName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := organizationDisplay.Ask(cmd, &inputs.DisplayName, nil); err != nil {
				return err
			}

			newOrg := &management.Organization{
				Name:        &inputs.Name,
				DisplayName: &inputs.DisplayName,
			}

			if inputs.Metadata != nil {
				newOrg.Metadata = &inputs.Metadata
			}

			branding := management.OrganizationBranding{}
			if inputs.LogoURL != "" {
				branding.LogoURL = &inputs.LogoURL
			}

			colors := make(map[string]string)
			if inputs.AccentColor != "" {
				colors[apiOrganizationColorPrimary] = inputs.AccentColor
			}
			if inputs.BackgroundColor != "" {
				colors[apiOrganizationColorPageBackground] = inputs.BackgroundColor
			}
			if len(colors) > 0 {
				branding.Colors = &colors
			}

			if branding.String() != "{}" {
				newOrg.Branding = &branding
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Organization.Create(cmd.Context(), newOrg)
			}); err != nil {
				return fmt.Errorf("failed to create organization with name %q: %w", inputs.Name, err)
			}

			cli.renderer.OrganizationCreate(newOrg)
			return nil
		},
	}

	organizationName.RegisterString(cmd, &inputs.Name, "")
	organizationDisplay.RegisterString(cmd, &inputs.DisplayName, "")
	organizationLogo.RegisterString(cmd, &inputs.LogoURL, "")
	organizationAccent.RegisterString(cmd, &inputs.AccentColor, "")
	organizationBackground.RegisterString(cmd, &inputs.BackgroundColor, "")
	organizationMetadata.RegisterStringMap(cmd, &inputs.Metadata, nil)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		DisplayName     string
		LogoURL         string
		AccentColor     string
		BackgroundColor string
		Metadata        map[string]string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an organization",
		Long: "Update an organization.\n\n" +
			"To update interactively, use `auth0 orgs update` with no arguments.\n\n" +
			"To update non-interactively, supply the organization id and " +
			"other information through the flags.",
		Example: `  auth0 orgs update <org-id>
  auth0 orgs update <org-id> --display "My Organization"
  auth0 orgs update <org-id> -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
  auth0 orgs update <org-id> -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions); err != nil {
					return err
				}
			}

			var oldOrg *management.Organization
			err := ansi.Waiting(func() (err error) {
				oldOrg, err = cli.api.Organization.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to read organization with ID %q: %w", inputs.ID, err)
			}

			if err := organizationDisplay.AskU(cmd, &inputs.DisplayName, oldOrg.DisplayName); err != nil {
				return err
			}

			if inputs.DisplayName == "" {
				inputs.DisplayName = oldOrg.GetDisplayName()
			}

			newOrg := &management.Organization{
				DisplayName: &inputs.DisplayName,
			}

			isLogoURLSet := len(inputs.LogoURL) > 0
			isAccentColorSet := len(inputs.AccentColor) > 0
			isBackgroundColorSet := len(inputs.BackgroundColor) > 0
			currentHasBranding := oldOrg.Branding != nil
			currentHasColors := currentHasBranding && oldOrg.Branding.Colors != nil
			needToAddColors := isAccentColorSet || isBackgroundColorSet || currentHasColors

			if isLogoURLSet || needToAddColors {
				newOrg.Branding = &management.OrganizationBranding{}

				if isLogoURLSet {
					newOrg.Branding.LogoURL = &inputs.LogoURL
				} else if currentHasBranding {
					newOrg.Branding.LogoURL = oldOrg.Branding.LogoURL
				}

				if needToAddColors {
					colors := make(map[string]string)

					if isAccentColorSet {
						colors[apiOrganizationColorPrimary] = inputs.AccentColor
					} else if currentHasColors && len(oldOrg.Branding.GetColors()[apiOrganizationColorPrimary]) > 0 {
						colors[apiOrganizationColorPrimary] = oldOrg.Branding.GetColors()[apiOrganizationColorPrimary]
					}

					if isBackgroundColorSet {
						colors[apiOrganizationColorPageBackground] = inputs.BackgroundColor
					} else if currentHasColors && len(oldOrg.Branding.GetColors()[apiOrganizationColorPageBackground]) > 0 {
						colors[apiOrganizationColorPageBackground] = oldOrg.Branding.GetColors()[apiOrganizationColorPageBackground]
					}

					newOrg.Branding.Colors = &colors
				}
			}

			newOrg.Metadata = oldOrg.Metadata
			if len(inputs.Metadata) != 0 {
				newOrg.Metadata = &inputs.Metadata
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Organization.Update(cmd.Context(), inputs.ID, newOrg)
			}); err != nil {
				return fmt.Errorf("failed to update organization with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.OrganizationUpdate(newOrg)

			return nil
		},
	}

	organizationDisplay.RegisterStringU(cmd, &inputs.DisplayName, "")
	organizationLogo.RegisterStringU(cmd, &inputs.LogoURL, "")
	organizationAccent.RegisterStringU(cmd, &inputs.AccentColor, "")
	organizationBackground.RegisterStringU(cmd, &inputs.BackgroundColor, "")
	organizationMetadata.RegisterStringMapU(cmd, &inputs.Metadata, nil)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteOrganizationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an organization",
		Long: "Delete an organization.\n\n" +
			"To delete interactively, use `auth0 orgs delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the organization id and the `--force` " +
			"flag to skip confirmation.",
		Example: `  auth0 orgs delete
  auth0 orgs rm
  auth0 orgs delete <org-id>
  auth0 orgs delete <org-id> --force
  auth0 orgs delete <org-id> <org-id2> <org-idn>
  auth0 orgs delete <org-id> <org-id2> <org-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			if len(args) == 0 {
				if err := organizationID.PickMany(cmd, &ids, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				ids = append(ids, args...)
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting organization(s)", ids, func(_ int, id string) error {
				if id != "" {
					if _, err := cli.api.Organization.Read(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete organization with ID %q: %w", id, err)
					}

					if err := cli.api.Organization.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete organization with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func openOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of an organization",
		Long:  "Open an organization's settings page in the Auth0 Dashboard.",
		Example: `  auth0 orgs open
  auth0 orgs open <org-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.Config.DefaultTenant, formatOrganizationDetailsPath(url.PathEscape(inputs.ID)))
			return nil
		},
	}

	return cmd
}

func membersOrganizationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage members of an organization",
		Long:  "Manage members of an organization.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listMembersOrganizationCmd(cli))

	return cmd
}

func listMembersOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List members of an organization",
		Long:    "List the members of an organization.",
		Example: `  auth0 orgs members list
  auth0 orgs members ls <org-id>
  auth0 orgs members list <org-id> --number 100
  auth0 orgs members ls <org-id> -n 100 --json
  auth0 orgs members ls <org-id> -n 100 --json-compact
  auth0 orgs members ls <org-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			members, err := cli.getOrgMembersWithSpinner(cmd.Context(), inputs.ID, inputs.Number)
			if err != nil {
				return err
			}

			sortMembers(members)

			cli.renderer.MembersList(members)

			return nil
		},
	}

	organizationNumber.Help = "Number of organization members to retrieve. Minimum 1, maximum 1000."
	organizationNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")
	cmd.SetUsageTemplate(resourceUsageTemplate())

	return cmd
}

func rolesOrganizationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage roles of an organization",
		Long: "Manage roles of an organization. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolesOrganizationCmd(cli))
	cmd.AddCommand(membersRolesOrganizationCmd(cli))

	return cmd
}

func listRolesOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		OrgID  string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List roles of an organization",
		Long:    "List roles assigned to members of an organization.",
		Example: `  auth0 orgs roles list
  auth0 orgs roles ls <org-id>
  auth0 orgs roles list <org-id> --number 100
  auth0 orgs roles ls <org-id> -n 100 --json
  auth0 orgs roles ls <org-id> -n 100 --json-compact
  auth0 orgs roles ls <org-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.OrgID = args[0]
			}
			members, err := cli.getOrgMembersWithSpinner(cmd.Context(), inputs.OrgID, inputs.Number)
			if err != nil {
				return err
			}

			roleMap, err := cli.getOrgMemberRolesWithSpinner(cmd.Context(), inputs.OrgID, members)
			if err != nil {
				return err
			}

			roles := cli.convertOrgRolesToManagementRoles(roleMap)

			cli.renderer.RoleList(roles)

			return nil
		},
	}

	organizationNumber.Help = "Number of organization roles to retrieve. Minimum 1, maximum 1000."
	organizationNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func membersRolesOrganizationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage roles of organization members",
		Long: "Each organization member can be assigned one or more roles, " +
			"which are applied when users log in through the organization.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listMembersRolesOrganizationCmd(cli))

	return cmd
}

func listMembersRolesOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		OrgID  string
		RoleID string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List organization members for a role",
		Long:    "List organization members that have a given role assigned to them.",
		Example: `  auth0 orgs roles members list
  auth0 orgs roles members ls <org-id>
  auth0 orgs roles members list <org-id> --role-id role
  auth0 orgs roles members list <org-id> --role-id role --number 100
  auth0 orgs roles members ls <org-id> -r role -n 100
  auth0 orgs roles members ls <org-id> -r role -n 100 --json
  auth0 orgs roles members ls <org-id> -r role -n 100 --json-compact
  auth0 orgs roles members ls <org-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions); err != nil {
					return err
				}
				if inputs.RoleID == "" {
					if err := roleID.Pick(cmd, &inputs.RoleID, cli.rolePickerOptions); err != nil {
						return err
					}
				}
			} else {
				inputs.OrgID = args[0]
			}

			members, err := cli.getOrgMembersWithSpinner(cmd.Context(), inputs.OrgID, inputs.Number)
			if err != nil {
				return err
			}

			roleMembers, err := cli.getOrgRoleMembersWithSpinner(cmd.Context(), inputs.OrgID, inputs.RoleID, members)
			if err != nil {
				return err
			}

			sortMembers(roleMembers)

			cli.renderer.MembersList(roleMembers)

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	roleIdentifier.RegisterString(cmd, &inputs.RoleID, "")
	organizationNumber.Help = "Number of members to retrieve. Minimum 1, maximum 1000."
	organizationNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func (cli *cli) organizationPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := cli.api.Organization.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Organizations {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no organizations to choose from. Create one by running: `auth0 orgs create`")
	}

	return opts, nil
}

func (cli *cli) invitationPickerOptions(ctx context.Context, orgID string) (pickerOptions, error) {
	orgInvitations, err := cli.api.Organization.Invitations(ctx, url.PathEscape(orgID))
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, inv := range orgInvitations.OrganizationInvitations {
		id := inv.GetID()
		label := fmt.Sprintf("%s %s", inv.Invitee.GetEmail(), ansi.Faint("("+id+")"))
		opts = append(opts, pickerOption{value: id, label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no invitations to choose from")
	}

	return opts, nil
}

func formatOrganizationDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("organizations/%s/overview", id)
}

func getWithPagination(
	limit int,
	api func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error),
) ([]interface{}, error) {
	var list []interface{}
	if err := ansi.Waiting(func() error {
		pageSize := defaultPageSize
		page := 0
		for {
			if limit > 0 {
				// Determine page size to avoid getting unwanted elements.
				want := limit - len(list)
				if want == 0 {
					return nil
				}
				if want < defaultPageSize {
					pageSize = want
				} else {
					pageSize = defaultPageSize
				}
			}
			res, hasNext, err := api(
				management.PerPage(pageSize),
				management.Page(page))
			if err != nil {
				return err
			}
			page++
			list = append(list, res...)
			if len(list) == limit || !hasNext {
				return nil
			}
		}
	}); err != nil {
		return nil, err
	}
	return list, nil
}

func (cli *cli) getOrgMembers(
	context context.Context,
	orgID string,
	number int,
) ([]management.OrganizationMember, error) {
	list, err := getWithPagination(
		number,
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			members, apiErr := cli.api.Organization.Members(context, url.PathEscape(orgID), opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, member := range members.Members {
				output = append(output, member)
			}
			return output, members.HasNext(), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list members of organization with ID %q: %w", orgID, err)
	}

	var typedList []management.OrganizationMember
	for _, item := range list {
		typedList = append(typedList, item.(management.OrganizationMember))
	}

	return typedList, nil
}

func sortMembers(members []management.OrganizationMember) {
	sort.Slice(members, func(i, j int) bool {
		return strings.ToLower(members[i].GetName()) < strings.ToLower(members[j].GetName())
	})
}

func (cli *cli) getOrgMembersWithSpinner(context context.Context, orgID string, number int,
) ([]management.OrganizationMember, error) {
	var members []management.OrganizationMember

	err := ansi.Waiting(func() (err error) {
		members, err = cli.getOrgMembers(context, orgID, number)
		return err
	})

	return members, err
}

func (cli *cli) getOrgMemberRolesWithSpinner(
	ctx context.Context,
	orgID string,
	members []management.OrganizationMember,
) (map[string]management.OrganizationMemberRole, error) {
	roleMap := make(map[string]management.OrganizationMemberRole)

	err := ansi.Waiting(func() (err error) {
		for _, member := range members {
			userID := member.GetUserID()

			roleList, err := cli.api.Organization.MemberRoles(ctx, orgID, userID)
			if err != nil {
				return err
			}

			for _, role := range roleList.Roles {
				roleID := role.GetID()
				if _, exists := roleMap[roleID]; !exists {
					roleMap[roleID] = role
				}
			}
		}

		return nil
	})

	return roleMap, err
}

func (cli *cli) convertOrgRolesToManagementRoles(
	roleMap map[string]management.OrganizationMemberRole,
) []*management.Role {
	var roles []*management.Role
	for _, role := range roleMap {
		roles = append(roles, &management.Role{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
		})
	}

	sort.Slice(roles, func(i, j int) bool {
		return strings.ToLower(roles[i].GetName()) < strings.ToLower(roles[j].GetName())
	})

	return roles
}

func (cli *cli) getOrgRoleMembersWithSpinner(
	ctx context.Context,
	orgID string,
	roleID string,
	members []management.OrganizationMember,
) ([]management.OrganizationMember, error) {
	var roleMembers []management.OrganizationMember

	err := ansi.Waiting(func() (err error) {
		for _, member := range members {
			userID := member.GetUserID()

			roleList, err := cli.api.Organization.MemberRoles(ctx, orgID, userID)
			if err != nil {
				return err
			}

			for _, role := range roleList.Roles {
				id := role.GetID()
				if id == roleID {
					roleMembers = append(roleMembers, member)
				}
			}
		}

		return nil
	})

	return roleMembers, err
}

func invitationsOrganizationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invitations",
		Aliases: []string{"invs"},
		Short:   "Manage invitations of an organization",
		Long:    "Manage invitations of an organization.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listInvitationsOrganizationCmd(cli))
	cmd.AddCommand(showInvitationOrganizationCmd(cli))
	cmd.AddCommand(createInvitationOrganizationCmd(cli))
	cmd.AddCommand(deleteInvitationOrganizationCmd(cli))

	return cmd
}

func showInvitationOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		OrgID        string
		InvitationID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(2),
		Short: "Show an organization invitation",
		Long:  "Display information about an organization invitation.",
		Example: `  auth0 orgs invs show
  auth0 orgs invs show <org-id>
  auth0 orgs invs show <org-id> <invitation-id>
  auth0 orgs invs show <org-id> <invitation-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.OrgID = args[0]
			}

			if len(args) <= 1 {
				if err := invitationID.Pick(cmd, &inputs.InvitationID, func(ctx context.Context) (pickerOptions, error) {
					return cli.invitationPickerOptions(ctx, inputs.OrgID)
				}); err != nil {
					return err
				}
			} else {
				inputs.InvitationID = args[1]
			}

			var invitation *management.OrganizationInvitation
			if err := ansi.Waiting(func() (err error) {
				invitation, err = cli.api.Organization.Invitation(cmd.Context(), inputs.OrgID, inputs.InvitationID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read organization invitation %q: %w", inputs.InvitationID, err)
			}

			cli.renderer.InvitationsShow(*invitation)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func listInvitationsOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List invitations of an organization",
		Long:    "List the invitations of an organization.",
		Example: `  auth0 orgs invs list
  auth0 orgs invs ls <org-id>
  auth0 orgs invs list <org-id> --number 100
  auth0 orgs invs ls <org-id> -n 100 --json
  auth0 orgs invs ls <org-id> -n 100 --json-compact
  auth0 orgs invs ls <org-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			invitations, err := cli.getOrgInvitationsWithSpinner(cmd.Context(), inputs.ID, inputs.Number)
			if err != nil {
				return err
			}

			sortInvitations(invitations)
			cli.renderer.InvitationsList(invitations)
			return nil
		},
	}

	organizationNumber.Help = "Number of organization invitations to retrieve. Minimum 1, maximum 1000."
	organizationNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func (cli *cli) getOrgInvitationsWithSpinner(context context.Context, orgID string, number int,
) ([]management.OrganizationInvitation, error) {
	var invitations []management.OrganizationInvitation

	err := ansi.Waiting(func() (err error) {
		invitations, err = cli.getOrgInvitations(context, orgID, number)
		return err
	})

	return invitations, err
}

func (cli *cli) getOrgInvitations(
	context context.Context,
	orgID string,
	number int,
) ([]management.OrganizationInvitation, error) {
	list, err := getWithPagination(
		number,
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			invitations, apiErr := cli.api.Organization.Invitations(context, url.PathEscape(orgID), opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, invitation := range invitations.OrganizationInvitations {
				if invitation != nil {
					output = append(output, *invitation)
				}
			}
			return output, invitations.HasNext(), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations of organization with ID %q: %w", orgID, err)
	}

	var typedList []management.OrganizationInvitation
	for _, item := range list {
		typedList = append(typedList, item.(management.OrganizationInvitation))
	}

	return typedList, nil
}

func sortInvitations(invitations []management.OrganizationInvitation) {
	sort.Slice(invitations, func(i, j int) bool {
		return strings.ToLower(invitations[i].GetCreatedAt()) < strings.ToLower(invitations[j].GetCreatedAt())
	})
}

func createInvitationOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		OrgID               string
		InviterName         string
		InviteeEmail        string
		ClientID            string
		ConnectionID        string
		TTLSeconds          int
		SendInvitationEmail bool
		Roles               []string
		AppMetadata         string
		UserMetadata        string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a new invitation to an organization",
		Long:  "Create a new invitation to an organization.",
		Example: `  auth0 orgs invs create
  auth0 orgs invs create <org-id>
  auth0 orgs invs create <org-id> --inviter-name "Inviter Name" --invitee-email "invitee@example.com" 
  auth0 orgs invs create <org-id> --invitee-email "invitee@example.com" --client-id "client_id"
  auth0 orgs invs create <org-id> -n "Inviter Name" -e "invitee@example.com" --client-id "client_id" -connection-id "connection_id" -t 86400
  auth0 orgs invs create <org-id> --json --inviter-name "Inviter Name"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.OrgID = args[0]
			}
			if err := clientID.Pick(cmd, &inputs.ClientID, cli.appPickerOptions()); err != nil {
				return err
			}
			if err := inviterName.Ask(cmd, &inputs.InviterName, nil); err != nil {
				return err
			}
			if err := inviteeEmail.Ask(cmd, &inputs.InviteeEmail, nil); err != nil {
				return err
			}

			invitation := &management.OrganizationInvitation{
				Inviter:             &management.OrganizationInvitationInviter{Name: &inputs.InviterName},
				Invitee:             &management.OrganizationInvitationInvitee{Email: &inputs.InviteeEmail},
				ClientID:            &inputs.ClientID,
				TTLSec:              &inputs.TTLSeconds,
				SendInvitationEmail: &inputs.SendInvitationEmail,
			}
			if inputs.ConnectionID != "" {
				invitation.ConnectionID = &inputs.ConnectionID
			}
			if inputs.AppMetadata != "" {
				var metadata map[string]interface{}
				if err := json.Unmarshal([]byte(inputs.AppMetadata), &metadata); err != nil {
					return fmt.Errorf("invalid JSON for app metadata: %w", err)
				}
				invitation.AppMetadata = metadata
			}
			if inputs.UserMetadata != "" {
				var metadata map[string]interface{}
				if err := json.Unmarshal([]byte(inputs.UserMetadata), &metadata); err != nil {
					return fmt.Errorf("invalid JSON for user metadata: %w", err)
				}
				invitation.UserMetadata = metadata
			}
			if len(inputs.Roles) > 0 {
				invitation.Roles = inputs.Roles
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Organization.CreateInvitation(cmd.Context(), inputs.OrgID, invitation)
			}); err != nil {
				return fmt.Errorf("failed to create invitation for organization with ID %q: %w", inputs.OrgID, err)
			}

			cli.renderer.InvitationsCreate(*invitation)
			return nil
		},
	}

	inviterName.RegisterString(cmd, &inputs.InviterName, "")
	inviteeEmail.RegisterString(cmd, &inputs.InviteeEmail, "")
	clientID.RegisterString(cmd, &inputs.ClientID, "")
	connectionID.RegisterString(cmd, &inputs.ConnectionID, "")
	ttlSeconds.RegisterInt(cmd, &inputs.TTLSeconds, 0)
	sendInvitationEmail.RegisterBool(cmd, &inputs.SendInvitationEmail, true)
	roles.RegisterStringSlice(cmd, &inputs.Roles, nil)
	applicationMetadata.RegisterString(cmd, &inputs.AppMetadata, "")
	userMetadata.RegisterString(cmd, &inputs.UserMetadata, "")

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteInvitationOrganizationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		OrgID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete invitation(s) from an organization",
		Long: "Delete invitation(s) from an organization.\n\n" +
			"To delete interactively, use `auth0 orgs invs delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the organization id, invitation id(s) and " +
			"the `--force` flag to skip confirmation.",
		Example: `  auth0 orgs invs delete
  auth0 orgs invs rm
  auth0 orgs invs delete <org-id> <invitation-id>
  auth0 orgs invs delete <org-id> <invitation-id> --force
  auth0 orgs invs delete <org-id> <inv-id1> <inv-id2> <inv-id3>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.OrgID = args[0]
				args = args[1:]
			}

			invitationIDs := make([]string, len(args))
			if len(args) == 0 {
				if err := invitationID.PickMany(
					cmd,
					&invitationIDs,
					func(ctx context.Context) (pickerOptions, error) {
						return cli.invitationPickerOptions(ctx, inputs.OrgID)
					},
				); err != nil {
					return err
				}
			} else {
				invitationIDs = append(invitationIDs, args...)
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting invitation(s)", invitationIDs, func(_ int, invitationID string) error {
				if invitationID != "" {
					if err := cli.api.Organization.DeleteInvitation(cmd.Context(), inputs.OrgID, invitationID); err != nil {
						return fmt.Errorf("failed to delete invitation with ID %q from organization %q: %w", invitationID, inputs.OrgID, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

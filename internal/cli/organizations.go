package cli

import (
	"context"
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

	roleIdentifier = Flag{
		Name:       "Role",
		LongForm:   "role-id",
		ShortForm:  "r",
		Help:       "Role Identifier.",
		IsRequired: true,
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
  auth0 orgs ls -n 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := getWithPagination(
				cmd.Context(),
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					res, err := cli.api.Organization.List(opts...)
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
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			var orgs []*management.Organization
			for _, item := range list {
				orgs = append(orgs, item.(*management.Organization))
			}

			cli.renderer.OrganizationList(orgs)

			return nil
		},
	}

	number.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
  auth0 orgs show <id>
  auth0 orgs show <id> --json`,
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
				return cli.api.Organization.Create(newOrg)
			}); err != nil {
				return fmt.Errorf("failed to create an organization with name '%s': %w", inputs.Name, err)
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
		Example: `  auth0 orgs update <id>
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

			var oldOrg *management.Organization
			err := ansi.Waiting(func() error {
				var err error
				oldOrg, err = cli.api.Organization.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to fetch organization with ID: %s %w", inputs.ID, err)
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
				return cli.api.Organization.Update(inputs.ID, newOrg)
			}); err != nil {
				return err
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
		Long: "Delete an organization.\n\n" +
			"To delete interactively, use `auth0 orgs delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the organization id and the `--force` " +
			"flag to skip confirmation.",
		Example: `  auth0 orgs delete
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
  auth0 orgs open <id>`,
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
  auth0 orgs members ls <id>
  auth0 orgs members ls <id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.ID, cli.organizationPickerOptions)
				if err != nil {
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

	number.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
  auth0 orgs roles ls <id>
  auth0 orgs roles ls <id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.OrgID = args[0]
			}

			members, err := cli.getOrgMembersWithSpinner(cmd.Context(), inputs.OrgID, inputs.Number)
			if err != nil {
				return err
			}
			roleMap, err := cli.getOrgMemberRolesWithSpinner(inputs.OrgID, members)
			if err != nil {
				return err
			}
			roles := cli.convertOrgRolesToManagementRoles(roleMap)
			cli.renderer.RoleList(roles)
			return nil
		},
	}

	number.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List organization members for a role",
		Long:  "List organization members that have a given role assigned to them.",
		Example: `  auth0 orgs roles members list
  auth0 orgs roles members list <org id> --role-id role`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := organizationID.Pick(cmd, &inputs.OrgID, cli.organizationPickerOptions)
				if err != nil {
					return err
				}
				if inputs.RoleID == "" {
					err = roleID.Pick(cmd, &inputs.RoleID, cli.rolePickerOptions)
					if err != nil {
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

			roleMembers, err := cli.getOrgRoleMembersWithSpinner(inputs.OrgID, inputs.RoleID, members)
			if err != nil {
				return err
			}
			sortMembers(roleMembers)
			cli.renderer.MembersList(roleMembers)
			return nil
		},
	}

	roleIdentifier.RegisterString(cmd, &inputs.RoleID, "")
	number.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func (cli *cli) organizationPickerOptions() (pickerOptions, error) {
	list, err := cli.api.Organization.List()
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

func getWithPagination(
	context context.Context,
	limit int,
	api func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error),
) ([]interface{}, error) {
	var list []interface{}
	if err := ansi.Waiting(func() error {
		pageSize := defaultPageSize
		page := 0
		for {
			if limit > 0 {
				// determine page size to avoid getting unwanted elements
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
				management.Context(context),
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
		context,
		number,
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			members, apiErr := cli.api.Organization.Members(url.PathEscape(orgID), opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, member := range members.Members {
				output = append(output, member)
			}
			return output, members.HasNext(), nil
		})

	if err != nil {
		return nil, fmt.Errorf("Unable to list members of an organization with Id '%s': %w", orgID, err)
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
	err := ansi.Spinner("Getting members of organization", func() error {
		var errInner error
		members, errInner = cli.getOrgMembers(context, orgID, number)
		return errInner
	})
	return members, err
}

func (cli *cli) getOrgMemberRolesWithSpinner(orgID string, members []management.OrganizationMember,
) (map[string]management.OrganizationMemberRole, error) {
	roleMap := make(map[string]management.OrganizationMemberRole)
	err := ansi.Spinner("Getting roles for each member", func() error {
		for _, member := range members {
			userID := member.GetUserID()
			roleList, errInner := cli.api.Organization.MemberRoles(orgID, userID)
			if errInner != nil {
				return errInner
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

func (cli *cli) convertOrgRolesToManagementRoles(roleMap map[string]management.OrganizationMemberRole,
) []*management.Role {
	var roles []*management.Role
	for _, role := range roleMap {
		roles = append(roles, &management.Role{ID: role.ID, Name: role.Name, Description: role.Description})
	}
	sort.Slice(roles, func(i, j int) bool {
		return strings.ToLower(roles[i].GetName()) < strings.ToLower(roles[j].GetName())
	})
	return roles
}

func (cli *cli) getOrgRoleMembersWithSpinner(orgID string, roleID string, members []management.OrganizationMember,
) ([]management.OrganizationMember, error) {
	var roleMembers []management.OrganizationMember
	errSpinner := ansi.Spinner("Getting roles assigned to organization members", func() error {
		for _, member := range members {
			userID := member.GetUserID()
			roleList, err := cli.api.Organization.MemberRoles(orgID, userID)
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
	return roleMembers, errSpinner
}

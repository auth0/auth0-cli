package cli

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
)

var (
	roleAPIIdentifier = Flag{
		Name:        "API",
		LongForm:    "api-id",
		ShortForm:   "a",
		Help:        "API Identifier.",
		IsRequired:  true,
		AlsoKnownAs: []string{"resource-server-identifier"},
	}

	roleAPIPermissions = Flag{
		Name:       "Permissions",
		LongForm:   "permissions",
		ShortForm:  "p",
		Help:       "Permissions.",
		IsRequired: true,
	}

	roleAPIPermissionsNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of permissions to retrieve. Minimum 1, maximum 1000.",
	}
)

func rolePermissionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permissions",
		Short:   "Manage permissions within the role resource",
		Long:    "Manage permissions within the role resource.",
		Aliases: []string{"perms"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolePermissionsCmd(cli))
	cmd.AddCommand(addRolePermissionsCmd(cli))
	cmd.AddCommand(removeRolePermissionsCmd(cli))

	return cmd
}

func listRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List permissions defined within a role",
		Long:    "List existing permissions defined in a role. To add a permission, run: `auth0 roles permissions add`.",
		Example: `  auth0 roles permissions list
  auth0 roles permissions ls <role-id>
  auth0 roles permissions ls <role-id> --number 100
  auth0 roles permissions ls <role-id> -n 100 --json
  auth0 roles permissions ls <role-id> -n 100 --json-compact
  auth0 roles permissions ls <role-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					permissionsList, err := cli.api.Role.Permissions(cmd.Context(), inputs.ID, opts...)
					if err != nil {
						return nil, false, err
					}

					for _, role := range permissionsList.Permissions {
						result = append(result, role)
					}
					return result, permissionsList.HasNext(), nil
				},
			)

			if err != nil {
				return fmt.Errorf("failed to read permissions for role with ID %q: %w", inputs.ID, err)
			}

			var permissions []*management.Permission
			for _, item := range list {
				permissions = append(permissions, item.(*management.Permission))
			}

			cli.renderer.RolePermissionList(permissions)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	roleAPIPermissionsNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func addRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		APIIdentifier string
		Permissions   []string
	}

	cmd := &cobra.Command{
		Use:   "add",
		Args:  cobra.MaximumNArgs(1),
		Short: "Add a permission to a role",
		Long:  "Add an existing permission defined in one of your APIs.",
		Example: `  auth0 roles permissions add
  auth0 roles permissions add <role-id>
  auth0 roles permissions add <role-id> --api-id <api-id>
  auth0 roles permissions add <role-id> --api-id <api-id> --permissions <permission-name>
  auth0 roles permissions add <role-id> -a <api-id> -p <permission-name>
  auth0 roles permissions add <role-id> --resource-server-identifier <api-id> --permissions <permission-name>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := roleAPIIdentifier.Pick(cmd, &inputs.APIIdentifier, cli.apiPickerOptionsWithoutAuth0); err != nil {
				return err
			}

			var rs *management.ResourceServer
			rs, err := cli.api.ResourceServer.Read(cmd.Context(), inputs.APIIdentifier)
			if err != nil {
				return fmt.Errorf("failed to read API with identifier %q: %w", inputs.APIIdentifier, err)
			}

			if len(inputs.Permissions) == 0 {
				err := cli.pickRolePermissions(rs.GetScopes(), &inputs.Permissions)
				if err != nil {
					return err
				}
			}

			ps := makePermissions(rs.GetIdentifier(), inputs.Permissions)
			if err := cli.api.Role.AssociatePermissions(cmd.Context(), inputs.ID, ps); err != nil {
				return fmt.Errorf("failed to associate permissions to role with ID %q: %w", inputs.ID, err)
			}

			role, err := cli.api.Role.Read(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read role with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RolePermissionAdd(role, rs, inputs.Permissions)

			return nil
		},
	}

	roleAPIIdentifier.RegisterString(cmd, &inputs.APIIdentifier, "")
	roleAPIPermissions.RegisterStringSlice(cmd, &inputs.Permissions, nil)

	return cmd
}

func removeRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		APIIdentifier string
		Permissions   []string
	}

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove a permission from a role",
		Long:    "Remove an existing permission defined in one of your APIs.",
		Example: `  auth0 roles permissions remove
  auth0 roles permissions rm <role-id> --api-id <api-id>
  auth0 roles permissions rm <role-id> --api-id <api-id> --permissions <permission-name>
  auth0 roles permissions rm <role-id> -a <api-id> -p <permission-name>
  auth0 roles permissions rm <role-id> --resource-server-identifier <api-id> --permissions <permission-name>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := roleAPIIdentifier.Pick(cmd, &inputs.APIIdentifier, cli.apiPickerOptionsWithoutAuth0); err != nil {
				return err
			}

			var rs *management.ResourceServer
			rs, err := cli.api.ResourceServer.Read(cmd.Context(), inputs.APIIdentifier)
			if err != nil {
				return fmt.Errorf("failed to read API with identifier %q: %w", inputs.APIIdentifier, err)
			}

			if len(inputs.Permissions) == 0 {
				err := cli.pickRolePermissions(rs.GetScopes(), &inputs.Permissions)
				if err != nil {
					return err
				}
			}

			ps := makePermissions(rs.GetIdentifier(), inputs.Permissions)
			if err := cli.api.Role.RemovePermissions(cmd.Context(), inputs.ID, ps); err != nil {
				return fmt.Errorf("failed to remove permissions from role with ID %q: %w", inputs.ID, err)
			}

			role, err := cli.api.Role.Read(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read role with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RolePermissionRemove(role, rs, inputs.Permissions)

			return nil
		},
	}

	roleAPIIdentifier.RegisterString(cmd, &inputs.APIIdentifier, "")
	roleAPIPermissions.RegisterStringSlice(cmd, &inputs.Permissions, nil)

	return cmd
}

func (c *cli) apiPickerOptionsWithoutAuth0(ctx context.Context) (pickerOptions, error) {
	return c.filteredAPIPickerOptions(ctx, func(r *management.ResourceServer) bool {
		parsedURL, err := url.Parse(r.GetIdentifier())
		if err != nil {
			return false
		}

		// We only allow API Identifiers not matching the
		// tenant domain, similar to the dashboard UX.
		return parsedURL.Host != c.tenant
	})
}

func (c *cli) pickRolePermissions(apiScopes []management.ResourceServerScope, permissions *[]string) error {
	// NOTE(cyx): We're inlining this for now since we have no generic
	// usecase for this particular picker type yet.
	var options []string
	for _, s := range apiScopes {
		options = append(options, s.GetValue())
	}

	p := &survey.MultiSelect{
		Message: "Permissions",
		Options: options,
	}

	err := survey.AskOne(p, permissions)

	return err
}

func makePermissions(id string, permissions []string) []*management.Permission {
	var result []*management.Permission
	for _, p := range permissions {
		result = append(result, &management.Permission{
			ResourceServerIdentifier: auth0.String(id),
			Name:                     auth0.String(p),
		})
	}
	return result
}

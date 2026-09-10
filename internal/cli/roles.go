package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	roleID = Argument{
		Name: "Role ID",
		Help: "Id of the role.",
	}
	roleName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the role.",
		IsRequired: true,
	}
	roleDescription = Flag{
		Name:       "Description",
		LongForm:   "description",
		ShortForm:  "d",
		Help:       "Description of the role.",
		IsRequired: true,
	}
	roleNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of roles to retrieve. Minimum 1, maximum 1000.",
	}
)

func rolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage resources for roles",
		Long: "Manage resources for roles. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolesCmd(cli))
	cmd.AddCommand(showRoleCmd(cli))
	cmd.AddCommand(createRoleCmd(cli))
	cmd.AddCommand(updateRoleCmd(cli))
	cmd.AddCommand(deleteRoleCmd(cli))
	cmd.AddCommand(rolePermissionsCmd(cli))

	return cmd
}

func listRolesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your roles",
		Long: `List your existing roles. To create one, run: ` + "`auth0 roles create`" + `.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).`,
		Example: `  auth0 roles list
  auth0 roles ls
  auth0 roles ls --number 100
  auth0 roles ls -n 100 --json
  auth0 roles ls -n 100 --json-compact
  auth0 roles ls --csv
  auth0 roles list --schema
  auth0 roles list --schema --json
  auth0 roles list --query '{"name_filter":"admin"}'
  auth0 roles list --query '{"name_filter":"admin"}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/roles")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "roles",
					SchemaCmd: "auth0 roles list",
				}, inputs.Query)
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					roleList, err := cli.api.Role.List(cmd.Context(), opts...)
					if err != nil {
						return nil, false, err
					}
					if roleList == nil {
						return result, false, nil
					}

					for _, role := range roleList.Roles {
						result = append(result, role)
					}

					return result, roleList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("failed to list roles: %w", err)
			}

			var roles []*management.Role
			for _, item := range list {
				roles = append(roles, item.(*management.Role))
			}

			cli.renderer.RoleList(roles)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	roleNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")

	return cmd
}

func showRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a role",
		Long:  "Display information about a role.",
		Example: `  auth0 roles show
  auth0 roles show <role-id>
  auth0 roles show <role-id> --json
  auth0 roles show <role-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			r := &management.Role{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				r, err = cli.api.Role.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read role with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RoleShow(r)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name        string
		Description string
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new role",
		Long: `Create a new role.

To create interactively, use ` + "`auth0 roles create`" + ` with no arguments.

To create non-interactively, supply the role name and description through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).`,
		Example: `  auth0 roles create
  auth0 roles create --name myrole --description "awesome role"
  auth0 roles create -n myrole -d "awesome role" --json-compact
  auth0 roles create -n myrole -d "awesome role" --json

  # Discover the payload schema
  auth0 roles create --schema
  auth0 roles create --schema --json

  # JSON input mode (for agents and automation)
  auth0 roles create --data '{"name":"myrole","description":"awesome role"}'
  auth0 roles create --data @role.json
  cat role.json | auth0 roles create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/roles")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createRoleFromJSON(cli, cmd, payload)
			}

			if err := roleName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := roleDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			r := &management.Role{
				Name:        &inputs.Name,
				Description: &inputs.Description,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Role.Create(cmd.Context(), r)
			}); err != nil {
				return fmt.Errorf("failed to create role: %w", err)
			}

			cli.renderer.RoleCreate(r)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	roleName.RegisterString(cmd, &inputs.Name, "")
	roleDescription.RegisterString(cmd, &inputs.Description, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a role",
		Long: `Update a role.

To update interactively, use ` + "`auth0 roles update`" + ` with no arguments.

To update non-interactively, supply the role id, name and description through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).`,
		Example: `  auth0 roles update
  auth0 roles update <role-id> --name myrole
  auth0 roles update <role-id> --name myrole --description "awesome role"
  auth0 roles update <role-id> -n myrole -d "awesome role" --json
  auth0 roles update <role-id> -n myrole -d "awesome role" --json-compact

  # Discover the payload schema
  auth0 roles update --schema
  auth0 roles update --schema --json

  # JSON input mode (for agents and automation)
  auth0 roles update <role-id> --data '{"name":"myrole","description":"awesome role"}'
  auth0 roles update <role-id> --data @role.json
  cat role.json | auth0 roles update <role-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/roles/{id}")
			}

			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateRoleFromJSON(cli, cmd, inputs.ID, payload)
			}

			var currentRole *management.Role
			if err := ansi.Waiting(func() (err error) {
				currentRole, err = cli.api.Role.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read role with ID %q: %w", inputs.ID, err)
			}

			if err := roleName.AskU(cmd, &inputs.Name, currentRole.Name); err != nil {
				return err
			}

			if err := roleDescription.AskU(cmd, &inputs.Description, currentRole.Description); err != nil {
				return err
			}

			updatedRole := &management.Role{}

			if inputs.Name != "" {
				updatedRole.Name = &inputs.Name
			}
			if inputs.Description != "" {
				updatedRole.Description = &inputs.Description
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Role.Update(cmd.Context(), inputs.ID, updatedRole)
			}); err != nil {
				return fmt.Errorf("failed to update role with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.RoleUpdate(updatedRole)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	roleName.RegisterStringU(cmd, &inputs.Name, "")
	roleDescription.RegisterStringU(cmd, &inputs.Description, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func deleteRoleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a role",
		Long: "Delete a role.\n\n" +
			"To delete interactively, use `auth0 roles delete`.\n\n" +
			"To delete non-interactively, supply the role id and the `--force` flag to skip confirmation.",
		Example: `  auth0 roles delete
  auth0 roles rm
  auth0 roles delete <role-id>
  auth0 roles delete <role-id> --force
  auth0 roles delete <role-id> <role-id2> <role-idn>
  auth0 roles delete <role-id> <role-id2> <role-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := roleID.PickMany(cmd, &ids, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting Role(s)", ids, func(_ int, id string) error {
				if id != "" {
					if _, err := cli.api.Role.Read(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete role with ID %q: %w", id, err)
					}

					if err := cli.api.Role.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete role with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func createRoleFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	role, err := runJSONWrite[management.Role](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/roles",
		URI:        cli.api.HTTPClient.URI("roles"),
		Data:       dataStr,
		SchemaCmd:  "auth0 roles create",
	})
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	cli.renderer.RoleCreate(role)
	return nil
}

func updateRoleFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	role, err := runJSONWrite[management.Role](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/roles/{id}",
		URI:        cli.api.HTTPClient.URI("roles", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 roles update",
	})
	if err != nil {
		return fmt.Errorf("failed to update role with ID %q: %w", id, err)
	}
	cli.renderer.RoleUpdate(role)
	return nil
}

func (c *cli) rolePickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Role.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions

	if list == nil {
		return nil, errors.New("there are currently no roles to choose from. Create one by running: `auth0 roles create`")
	}

	for _, c := range list.Roles {
		value := c.GetID()
		label := fmt.Sprintf("%s %s", c.GetName(), ansi.Faint("("+value+")"))
		opts = append(opts, pickerOption{value: value, label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no roles to choose from. Create one by running: `auth0 roles create`")
	}

	return opts, nil
}

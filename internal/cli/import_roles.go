package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/go-auth0/management"
)

type Role = management.Role

// Put here the Roles handler logic

// 1. Fetch all existing items
// 2. Map all YAML items to the corresponding Go SDK models
// 3. Distribute in create, delete, update and conflicts maps
// 4. Perform patch diff
// 5. Execute creates, updates and deletes

// 1. Fetch all existing items

func (cli *cli) getAllRoles(context context.Context) ([]*Role, error) {
	list, err := getWithPagination(
		context,
		0, // Get *all* roles
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			roles, apiErr := cli.api.Role.List(opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, role := range roles.Roles {
				output = append(output, role)
			}
			return output, roles.HasNext(), nil
		})

	if err != nil {
		return nil, fmt.Errorf("Unable to list roles: %w", err)
	}
	var typedList []*Role
	for _, item := range list {
		typedList = append(typedList, item.(*Role))
	}
	return typedList, nil
}

// 2. Map all YAML items to the corresponding Go SDK models

func mapYamlToRoles(yaml *TenantConfig) []*Role {
	var roles []*Role
	for _, yamlRole := range yaml.Roles {
		name := yamlRole.Name
		description := yamlRole.Description
		roles = append(roles, &Role{
			Name: &name,
			Description: &description,
		})
	}
	return roles
}

// 3. Distribute in create, delete, update and conflicts maps

func distributeRoleOperations(existingRoles []*Role, newRoles []*Role) (creates map[string]*Role, updates map[string]*Role, deletes map[string]*Role) {
	creates = make(map[string]*Role)
	for _, newRole := range newRoles {
		creates[newRole.GetName()] = newRole
	}

	updates = make(map[string]*Role)
	deletes = make(map[string]*Role)

	for _, existingRole := range existingRoles {
		// If the existing role matches a new role
		if _, ok := creates[existingRole.GetName()]; ok {
			// Add it to updates
			updates[existingRole.GetName()] = existingRole
		} else {
			// Add it to deletes
			deletes[existingRole.GetName()] = existingRole
		}
	}

	// Delete the ones to be updated from creates
	for _, update := range updates {
		delete(creates, update.GetName());
	}	

	// TODO: Handle conflicts
	return creates, updates, deletes
}

// 4. Perform patch diff

func diffRoles(config *ImportConfig, existingRoles map[string]*Role, newRoles map[string]*Role) map[string]*Role { // existingRoles -> updates, newRoles -> creates
	for _, existingRole := range existingRoles {
		newRole := newRoles[existingRole.GetName()]
		existingRoles[existingRole.GetName()] = diffRole(config, existingRole, newRole)
	}
	return existingRoles
}

func diffRole(config *ImportConfig, existingRole *Role, newRole *Role) *Role { // existingRole -> update, newRole -> create
	if newRole == nil {
		return existingRole
	}
	if newRole.Description == nil && config.Auth0AllowDelete {
		existingRole.Description = nil
	} else if existingRole.GetDescription() != newRole.GetDescription() {
		existingRole.Description = newRole.Description
	}
	return existingRole
}

// 5. Execute creates, updates and deletes

func roleToJSON(role *Role) string {
	json, err := json.Marshal(role)
	if err != nil {
		return role.GetName()
	}
	return string(json)
}

func (cli *cli) createRoles(roles []*Role) error {
	for _, role := range roles {
		err := cli.api.Role.Create(role)
		if err != nil {
			return fmt.Errorf("Unable to create role '%s': %w", role.GetName(), err)
		}
		cli.renderer.Infof("Created role: %s", roleToJSON(role))
	}
	return nil
}

func (cli *cli) updateRoles(roles []*Role) error {
	for _, role := range roles {
		id := role.GetID()
		role.ID = nil
		err := cli.api.Role.Update(id, role)
		if err != nil {
			return fmt.Errorf("Unable to update role '%s': %w", role.GetName(), err)
		}
		cli.renderer.Infof("Updated role: %s", roleToJSON(role))
	}
	return nil
}

func (cli *cli) deleteRoles(roles []*Role) error {
	for _, role := range roles {
		err := cli.api.Role.Delete(role.GetID())
		if err != nil {
			return fmt.Errorf("Unable to delete role '%s': %w", role.GetName(), err)
		}
		cli.renderer.Infof("Deleted role: %s", roleToJSON(role))
	}
	return nil
}

func processRoleOperations(cli *cli, creates []*Role, updates []*Role, deletes []*Role) error {
	if err := cli.deleteRoles(deletes); err != nil {
		return err
	}
	if err := cli.updateRoles(updates); err != nil {
		return err
	}
	if err := cli.createRoles(creates); err != nil {
		return err
	}

	return nil
}

func rolesMapToSlice(m map[string]*Role) []*Role {
	var roles []*Role
	for _, role := range m {
		roles = append(roles, role)
	}
	return roles
}

// Bring it all together
 
func ImportRoles(ctx context.Context, cli *cli, config *ImportConfig, yaml *TenantConfig) (changes *auth0.ImportChanges, err error) {
	// 1. Fetch all existing items
	existingRoles, err := cli.getAllRoles(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Map all YAML items to the corresponding Go SDK models
	newRoles := mapYamlToRoles(yaml)

	// 3. Distribute in create, delete, update and conflicts maps
	createsMap, updatesMap, deletesMap := distributeRoleOperations(existingRoles, newRoles)

	// 4. Perform patch diff
	updatesMap = diffRoles(config, updatesMap, createsMap)

	// 5. Execute creates, updates and deletes
	createsSlice := rolesMapToSlice(createsMap)
	updatesSlice := rolesMapToSlice(updatesMap)
    deletesSlice := rolesMapToSlice(deletesMap)

	err = processRoleOperations(cli, createsSlice, updatesSlice, deletesSlice)
	if err != nil {
		return nil, err
	}

	result := auth0.ImportChanges{
		Resource: "Roles", 
		Creates: len(createsSlice), 
		Updates: len(updatesSlice), 
		Deletes: len(deletesSlice),
	}

	return &result, nil
}

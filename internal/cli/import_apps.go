package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/go-auth0/management"
)

type App = management.Client

// Put here the Apps handler logic

// 1. Fetch all existing items
// 2. Map all YAML items to the corresponding Go SDK models
// 3. Distribute in create, delete, update and conflicts maps
// 4. Perform patch diff
// 5. Execute creates, updates and deletes

// 1. Fetch all existing items

func (cli *cli) getAllApps(context context.Context) ([]*App, error) {
	list, err := getWithPagination(
		context,
		0, // Get *all* apps
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			apps, apiErr := cli.api.Client.List(opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, app := range apps.Clients {
				output = append(output, app)
			}
			return output, apps.HasNext(), nil
		})

	if err != nil {
		return nil, fmt.Errorf("Unable to list apps: %w", err)
	}
	var typedList []*App
	for _, item := range list {
		typedList = append(typedList, item.(*App))
	}
	return typedList, nil
}

// 2. Map all YAML items to the corresponding Go SDK models

func mapYamlToApps(yaml *TenantConfig) []*App {
	var apps []*App
	for _, yamlApp := range yaml.Clients {
		name := yamlApp.Name
		description := yamlApp.Description
		isFirstParty := yamlApp.IsFirstParty
		isTokenEndpointIpHeaderTrusted := yamlApp.IsTokenEndpointIPHeaderTrusted
		oidcConformant := yamlApp.OIDCConformant
		callbacks := stringToInterfaceSlice(yamlApp.Callbacks)
		allowedOrigins := stringToInterfaceSlice(yamlApp.AllowedOrigins)
		webOrigins := stringToInterfaceSlice(yamlApp.WebOrigins)
		allowedLogoutURLs := stringToInterfaceSlice(yamlApp.AllowedLogoutURLs)
		grantTypes := stringToInterfaceSlice(yamlApp.GrantTypes)
		ssoDisabled := yamlApp.SSODisabled
		crossOriginAuth := yamlApp.CrossOriginAuth
		customLoginPageOn := yamlApp.CustomLoginPageOn
		tokenEndpointAuthMethod := yamlApp.TokenEndpointAuthMethod

		app := &App{
			Name: &name,
			Description: &description,
			AppType: &yamlApp.AppType,
			IsFirstParty: &isFirstParty,
			IsTokenEndpointIPHeaderTrusted: &isTokenEndpointIpHeaderTrusted,
			OIDCConformant: &oidcConformant,
			Callbacks: callbacks,
			AllowedOrigins: allowedOrigins,
			WebOrigins: webOrigins,
			AllowedLogoutURLs: allowedLogoutURLs,
			GrantTypes: grantTypes,
			SSODisabled: &ssoDisabled,
			CrossOriginAuth: &crossOriginAuth,
			CustomLoginPageOn: &customLoginPageOn,
			TokenEndpointAuthMethod: &tokenEndpointAuthMethod,
		}
		if tokenEndpointAuthMethod == "" {
			app.TokenEndpointAuthMethod = nil
		}
		apps = append(apps, app)
	}
	return apps
}

// 3. Distribute in create, delete, update and conflicts maps

func distributeAppOperations(existingApps []*App, newApps []*App) (creates map[string]*App, updates map[string]*App, deletes map[string]*App) {
	creates = make(map[string]*App)
	for _, newApp := range newApps {
		creates[newApp.GetName()] = newApp
	}

	updates = make(map[string]*App)
	deletes = make(map[string]*App)

	for _, existingApp := range existingApps {
		// If the existing app matches a new app
		if _, ok := creates[existingApp.GetName()]; ok {
			// Add it to updates
			updates[existingApp.GetName()] = existingApp
		} else {
			// Add it to deletes
			deletes[existingApp.GetName()] = existingApp
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

func diffApps(config *ImportConfig, existingApps map[string]*App, newApps map[string]*App) map[string]*App { // existingApps -> updates, newApps -> creates
	for _, existingApp := range existingApps {
		newApp := newApps[existingApp.GetName()]
		existingApps[existingApp.GetName()] = diffApp(config, existingApp, newApp)
	}
	return existingApps
}

func diffApp(config *ImportConfig, existingApp *App, newApp *App) *App { // existingApp -> update, newApp -> create
	if newApp == nil {
		return existingApp
	}

	if newApp.Description == nil && config.Auth0AllowDelete {
		existingApp.Description = nil
	} else if existingApp.GetDescription() != newApp.GetDescription() {
		existingApp.Description = newApp.Description
	}

	if newApp.AppType == nil && config.Auth0AllowDelete {
		existingApp.AppType = nil
	} else if existingApp.GetAppType() != newApp.GetAppType() {
		existingApp.AppType = newApp.AppType
	}

	if newApp.IsFirstParty == nil && config.Auth0AllowDelete {
		existingApp.IsFirstParty = nil
	} else if existingApp.GetIsFirstParty() != newApp.GetIsFirstParty() {
		existingApp.IsFirstParty = newApp.IsFirstParty
	}

	if newApp.IsTokenEndpointIPHeaderTrusted == nil && config.Auth0AllowDelete {
		existingApp.IsTokenEndpointIPHeaderTrusted = nil
	} else if existingApp.GetIsTokenEndpointIPHeaderTrusted() != newApp.GetIsTokenEndpointIPHeaderTrusted() {
		existingApp.IsTokenEndpointIPHeaderTrusted = newApp.IsTokenEndpointIPHeaderTrusted
	}

	if newApp.OIDCConformant == nil && config.Auth0AllowDelete {
		existingApp.OIDCConformant = nil
	} else if existingApp.GetOIDCConformant() != newApp.GetOIDCConformant() {
		existingApp.OIDCConformant = newApp.OIDCConformant
	}

	if newApp.SSODisabled == nil && config.Auth0AllowDelete {
		existingApp.SSODisabled = nil
	} else if existingApp.GetSSODisabled() != newApp.GetSSODisabled() {
		existingApp.SSODisabled = newApp.SSODisabled
	}

	if newApp.CrossOriginAuth == nil && config.Auth0AllowDelete {
		existingApp.CrossOriginAuth = nil
	} else if existingApp.GetCrossOriginAuth() != newApp.GetCrossOriginAuth() {
		existingApp.CrossOriginAuth = newApp.CrossOriginAuth
	}

	if newApp.CustomLoginPageOn == nil && config.Auth0AllowDelete {
		existingApp.CustomLoginPageOn = nil
	} else if existingApp.GetCustomLoginPageOn() != newApp.GetCustomLoginPageOn() {
		existingApp.CustomLoginPageOn = newApp.CustomLoginPageOn
	}

	if newApp.TokenEndpointAuthMethod == nil && config.Auth0AllowDelete {
		existingApp.TokenEndpointAuthMethod = nil
	} else if existingApp.GetTokenEndpointAuthMethod() != newApp.GetTokenEndpointAuthMethod() {
		existingApp.TokenEndpointAuthMethod = newApp.TokenEndpointAuthMethod
	}

	if newApp.Callbacks == nil && config.Auth0AllowDelete {
		existingApp.Callbacks = nil
	} else {
		if len(existingApp.Callbacks) != len(newApp.Callbacks) {
			existingApp.Callbacks = newApp.Callbacks
		} else {
			for i := 0; i < len(existingApp.Callbacks); i++ {
				if existingApp.Callbacks[i] != newApp.Callbacks[i] {
					existingApp.Callbacks = newApp.Callbacks
					break
				}
			}
		}
	}

	if newApp.AllowedOrigins == nil && config.Auth0AllowDelete {
		existingApp.AllowedOrigins = nil
	} else {
		if len(existingApp.AllowedOrigins) != len(newApp.AllowedOrigins) {
			existingApp.AllowedOrigins = newApp.AllowedOrigins
		} else {
			for i := 0; i < len(existingApp.AllowedOrigins); i++ {
				if existingApp.AllowedOrigins[i] != newApp.AllowedOrigins[i] {
					existingApp.AllowedOrigins = newApp.AllowedOrigins
					break
				}
			}
		}
	}

	if newApp.WebOrigins == nil && config.Auth0AllowDelete {
		existingApp.WebOrigins = nil
	} else {
		if len(existingApp.WebOrigins) != len(newApp.WebOrigins) {
			existingApp.WebOrigins = newApp.WebOrigins
		} else {
			for i := 0; i < len(existingApp.WebOrigins); i++ {
				if existingApp.WebOrigins[i] != newApp.WebOrigins[i] {
					existingApp.WebOrigins = newApp.WebOrigins
					break
				}
			}
		}
	}

	if newApp.AllowedLogoutURLs == nil && config.Auth0AllowDelete {
		existingApp.AllowedLogoutURLs = nil
	} else {
		if len(existingApp.AllowedLogoutURLs) != len(newApp.AllowedLogoutURLs) {
			existingApp.AllowedLogoutURLs = newApp.AllowedLogoutURLs
		} else {
			for i := 0; i < len(existingApp.AllowedLogoutURLs); i++ {
				if existingApp.AllowedLogoutURLs[i] != newApp.AllowedLogoutURLs[i] {
					existingApp.AllowedLogoutURLs = newApp.AllowedLogoutURLs
					break
				}
			}
		}
	}

	if newApp.GrantTypes == nil && config.Auth0AllowDelete {
		existingApp.GrantTypes = nil
	} else {
		if len(existingApp.GrantTypes) != len(newApp.GrantTypes) {
			existingApp.GrantTypes = newApp.GrantTypes
		} else {
			for i := 0; i < len(existingApp.GrantTypes); i++ {
				if existingApp.GrantTypes[i] != newApp.GrantTypes[i] {
					existingApp.GrantTypes = newApp.GrantTypes
					break
				}
			}
		}
	}
	return existingApp
}

// 5. Execute creates, updates and deletes

func appToJSON(app *App) string {
	printableApp := App{
		Name: app.Name,
		ClientID: app.ClientID,
	}
	json, err := json.Marshal(printableApp)
	if err != nil {
		return app.GetName()
	}
	return string(json)
}

func (cli *cli) createApps(apps []*App) error {
	for _, app := range apps {
		err := cli.api.Client.Create(app)
		if err != nil {
			return fmt.Errorf("Unable to create application '%s': %w", app.GetName(), err)
		}
		cli.renderer.Infof("Created application: %s", appToJSON(app))
	}
	return nil
}

func (cli *cli) updateApps(apps []*App) error {
	for _, app := range apps {
		id := app.GetClientID()
		app.ClientID = nil
		app.ClientSecret =  nil
		app.SigningKeys = nil
		if app.JWTConfiguration != nil {
			app.JWTConfiguration.SecretEncoded = nil
		}
		err := cli.api.Client.Update(id, app)
		if err != nil {
			return fmt.Errorf("Unable to update application '%s': %w", app.GetName(), err)
		}
		cli.renderer.Infof("Updated application: %s", appToJSON(app))
	}
	return nil
}

func (cli *cli) deleteApps(apps []*App) error {
	for _, app := range apps {
		err := cli.api.Client.Delete(app.GetClientID())
		if err != nil {
			return fmt.Errorf("Unable to delete application '%s': %w", app.GetName(), err)
		}
		cli.renderer.Infof("Deleted application: %s", appToJSON(app))
	}
	return nil
}

func processAppOperations(cli *cli, creates []*App, updates []*App, deletes []*App) error {
	if err := cli.deleteApps(deletes); err != nil {
		return err
	}
	if err := cli.updateApps(updates); err != nil {
		return err
	}
	if err := cli.createApps(creates); err != nil {
		return err
	}

	return nil
}

func appsMapToSlice(m map[string]*App) []*App {
	var apps []*App
	for _, app := range m {
		apps = append(apps, app)
	}
	return apps
}

// Bring it all together

func ImportApps(ctx context.Context, cli *cli, config *ImportConfig, yaml *TenantConfig) (changes *auth0.ImportChanges, err error) {
	// 1. Fetch all existing items
	existingApps, err := cli.getAllApps(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Map all YAML items to the corresponding Go SDK models
	newApps := mapYamlToApps(yaml)

	// 3. Distribute in create, delete, update and conflicts maps
	createsMap, updatesMap, deletesMap := distributeAppOperations(existingApps, newApps)
	delete(deletesMap, "All Applications")

	// 4. Perform patch diff
	updatesMap = diffApps(config, updatesMap, createsMap)

	// 5. Execute creates, updates and deletes
	createsSlice := appsMapToSlice(createsMap)
	updatesSlice := appsMapToSlice(updatesMap)
    deletesSlice := appsMapToSlice(deletesMap)

	err = processAppOperations(cli, createsSlice, updatesSlice, deletesSlice)
	if err != nil {
		return nil, err
	}

	result := auth0.ImportChanges{
		Resource: "Applications", 
		Creates: len(createsSlice), 
		Updates: len(updatesSlice), 
		Deletes: len(deletesSlice),
	}

	return &result, nil
}

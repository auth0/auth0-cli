package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/go-auth0/management"
)

type API = management.ResourceServer

// Put here the APIs handler logic

// 1. Fetch all existing items
// 2. Map all YAML items to the corresponding Go SDK models
// 3. Distribute in create, delete, update and conflicts maps
// 4. Perform patch diff
// 5. Execute creates, updates and deletes

// 1. Fetch all existing items

func (cli *cli) getAllAPIs(context context.Context) ([]*API, error) {
	list, err := getWithPagination(
		context,
		0, // Get *all* apis
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, apiErr error) {
			apis, apiErr := cli.api.ResourceServer.List(opts...)
			if apiErr != nil {
				return nil, false, apiErr
			}
			var output []interface{}
			for _, api := range apis.ResourceServers {
				output = append(output, api)
			}
			return output, apis.HasNext(), nil
		})

	if err != nil {
		return nil, fmt.Errorf("Unable to list APIs: %w", err)
	}
	var typedList []*API
	for _, item := range list {
		typedList = append(typedList, item.(*API))
	}
	return typedList, nil
}

// 2. Map all YAML items to the corresponding Go SDK models

func mapYamlToAPIs(yaml *TenantConfig) []*API {
	var apis []*API
	for _, yamlAPI := range yaml.ResourceServers {
		var scopes []*management.ResourceServerScope
		for _, yamlScope := range yamlAPI.Scopes {
			value := yamlScope.Value
			description := yamlScope.Description
			scopes = append(scopes, &management.ResourceServerScope{
				Value: &value,
				Description: &description,
			})
		}
		name := yamlAPI.Name
		identifier := yamlAPI.Identifier
		signingAlgorithm := yamlAPI.SigningAlg
		signingSecret := yamlAPI.SigningSecret
		allowOfflineAccess := yamlAPI.AllowOfflineAccess
		tokenLifetime := yamlAPI.TokenLifetime
		tokenLifetimeForWeb := yamlAPI.TokenLifetimeForWeb
		skipConsentForVerifiableFirstPartyClients := yamlAPI.SkipConsentForVerifiableFirstPartyClients
		enforcePolicies := yamlAPI.EnforcePolicies
		tokenDialect := yamlAPI.TokenDialect

		api := &API{
			Name: &name,
			Identifier: &identifier,
			Scopes: scopes,
			SigningAlgorithm: &signingAlgorithm,
			SigningSecret: &signingSecret,
			AllowOfflineAccess: &allowOfflineAccess,
			TokenLifetime: &tokenLifetime,
			TokenLifetimeForWeb: &tokenLifetimeForWeb,
			SkipConsentForVerifiableFirstPartyClients: &skipConsentForVerifiableFirstPartyClients,
			EnforcePolicies: &enforcePolicies,
			TokenDialect: &tokenDialect,
		}
		if tokenDialect == "" {
			api.TokenDialect = nil
		}
		if signingSecret == "" {
			api.SigningSecret = nil
		}
		apis = append(apis, api)
	}
	return apis
}

// 3. Distribute in create, delete, update and conflicts maps

func distributeAPIOperations(existingAPIs []*API, newAPIs []*API) (creates map[string]*API, updates map[string]*API, deletes map[string]*API) {
	creates = make(map[string]*API)
	for _, newAPI := range newAPIs {
		creates[newAPI.GetIdentifier()] = newAPI
	}

	updates = make(map[string]*API)
	deletes = make(map[string]*API)

	for _, existingAPI := range existingAPIs {
		// If the existing api matches a new api
		if _, ok := creates[existingAPI.GetIdentifier()]; ok {
			// Add it to updates
			updates[existingAPI.GetIdentifier()] = existingAPI
		} else {
			// Add it to deletes
			deletes[existingAPI.GetIdentifier()] = existingAPI
		}
	}

	// Delete the ones to be updated from creates
	for _, update := range updates {
		delete(creates, update.GetIdentifier());
	}

	// TODO: Handle conflicts
	return creates, updates, deletes
}

// 4. Perform patch diff

func diffAPIs(config *ImportConfig, existingAPIs map[string]*API, newAPIs map[string]*API) map[string]*API { // existingAPIs -> updates, newAPIs -> creates
	for _, existingAPI := range existingAPIs {
		newAPI := newAPIs[existingAPI.GetIdentifier()]
		existingAPIs[existingAPI.GetIdentifier()] = diffAPI(config, existingAPI, newAPI)
	}
	return existingAPIs
}

func diffAPI(config *ImportConfig, existingAPI *API, newAPI *API) *API { // existingAPI -> update, newAPI -> create
	if newAPI == nil {
		return existingAPI
	}

	if newAPI.Name == nil && config.Auth0AllowDelete {
		existingAPI.Name = nil
	} else if existingAPI.GetName() != newAPI.GetName() {
		existingAPI.Name = newAPI.Name
	}

	if newAPI.Scopes == nil && config.Auth0AllowDelete {
		existingAPI.Scopes = nil
	} else {
		if len(existingAPI.Scopes) != len(newAPI.Scopes) {
			existingAPI.Scopes = newAPI.Scopes
		} else {
			for i := 0; i < len(existingAPI.Scopes); i++ {
				if (existingAPI.Scopes[i].GetDescription() != newAPI.Scopes[i].GetDescription()) || 
				(existingAPI.Scopes[i].GetValue() != newAPI.Scopes[i].GetValue()) {
					existingAPI.Scopes = newAPI.Scopes
					break
				}
			}
		}
	}

	if newAPI.SigningAlgorithm == nil && config.Auth0AllowDelete {
		existingAPI.SigningAlgorithm = nil
	} else if existingAPI.GetSigningAlgorithm() != newAPI.GetSigningAlgorithm() {
		existingAPI.SigningAlgorithm = newAPI.SigningAlgorithm
	}

	if newAPI.SigningSecret == nil && config.Auth0AllowDelete {
		existingAPI.SigningSecret = nil
	} else if existingAPI.GetSigningSecret() != newAPI.GetSigningSecret() {
		existingAPI.SigningSecret = newAPI.SigningSecret
	}

	if newAPI.AllowOfflineAccess == nil && config.Auth0AllowDelete {
		existingAPI.AllowOfflineAccess = nil
	} else if existingAPI.GetAllowOfflineAccess() != newAPI.GetAllowOfflineAccess() {
		existingAPI.AllowOfflineAccess = newAPI.AllowOfflineAccess
	}

	if newAPI.TokenLifetime == nil && config.Auth0AllowDelete {
		existingAPI.TokenLifetime = nil
	} else if existingAPI.GetTokenLifetime() != newAPI.GetTokenLifetime() {
		existingAPI.TokenLifetime = newAPI.TokenLifetime
	}

	if newAPI.TokenLifetimeForWeb == nil && config.Auth0AllowDelete {
		existingAPI.TokenLifetimeForWeb = nil
	} else if existingAPI.GetTokenLifetimeForWeb() != newAPI.GetTokenLifetimeForWeb() {
		existingAPI.TokenLifetimeForWeb = newAPI.TokenLifetimeForWeb
	}

	if newAPI.SkipConsentForVerifiableFirstPartyClients == nil && config.Auth0AllowDelete {
		existingAPI.SkipConsentForVerifiableFirstPartyClients = nil
	} else if existingAPI.GetSkipConsentForVerifiableFirstPartyClients() != newAPI.GetSkipConsentForVerifiableFirstPartyClients() {
		existingAPI.SkipConsentForVerifiableFirstPartyClients = newAPI.SkipConsentForVerifiableFirstPartyClients
	}

	if newAPI.EnforcePolicies == nil && config.Auth0AllowDelete {
		existingAPI.EnforcePolicies = nil
	} else if existingAPI.GetEnforcePolicies() != newAPI.GetEnforcePolicies() {
		existingAPI.EnforcePolicies = newAPI.EnforcePolicies
	}

	if newAPI.TokenDialect == nil && config.Auth0AllowDelete {
		existingAPI.TokenDialect = nil
	} else if existingAPI.GetTokenDialect() != newAPI.GetTokenDialect() {
		existingAPI.TokenDialect = newAPI.TokenDialect
	}

	return existingAPI
}

// 5. Execute creates, updates and deletes

func apiToJSON(api *API) string {
	printableAPI := API{
		Name: api.Name,
		Identifier: api.Identifier,
	}
	json, err := json.Marshal(printableAPI)
	if err != nil {
		return api.GetName()
	}
	return string(json)
}

func (cli *cli) createAPIs(apis []*API) error {
	for _, api := range apis {
		err := cli.api.ResourceServer.Create(api)
		if err != nil {
			return fmt.Errorf("Unable to create API '%s': %w", api.GetName(), err)
		}
		cli.renderer.Infof("Created API: %s", apiToJSON(api))
	}
	return nil
}

func (cli *cli) updateAPIs(apis []*API) error {
	for _, api := range apis {
		id := api.GetID()
		api.ID = nil
		api.Identifier = nil
		err := cli.api.ResourceServer.Update(id, api)
		if err != nil {
			return fmt.Errorf("Unable to update API '%s': %w", api.GetName(), err)
		}
		cli.renderer.Infof("Updated API: %s", apiToJSON(api))
	}
	return nil
}

func (cli *cli) deleteAPIs(apis []*API) error {
	for _, api := range apis {
		err := cli.api.ResourceServer.Delete(api.GetID())
		if err != nil {
			return fmt.Errorf("Unable to delete API '%s': %w", api.GetName(), err)
		}
		cli.renderer.Infof("Deleted API: %s", apiToJSON(api))
	}
	return nil
}

func processAPIOperations(cli *cli, creates []*API, updates []*API, deletes []*API) error {
	if err := cli.deleteAPIs(deletes); err != nil {
		return err
	}
	if err := cli.updateAPIs(updates); err != nil {
		return err
	}
	if err := cli.createAPIs(creates); err != nil {
		return err
	}

	return nil
}

func apisMapToSlice(m map[string]*API) []*API {
	var apis []*API
	for _, api := range m {
		apis = append(apis, api)
	}
	return apis
}

// Bring it all together

func ImportAPIs(ctx context.Context, cli *cli, config *ImportConfig, yaml *TenantConfig) (changes *auth0.ImportChanges, err error) {
	// 1. Fetch all existing items
	existingAPIs, err := cli.getAllAPIs(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Map all YAML items to the corresponding Go SDK models
	newAPIs := mapYamlToAPIs(yaml)

	// 3. Distribute in create, delete, update and conflicts maps
	createsMap, updatesMap, deletesMap := distributeAPIOperations(existingAPIs, newAPIs)

	managementAPIIdentifier := fmt.Sprintf("https://%s/api/v2/", cli.tenant)
	delete(deletesMap, managementAPIIdentifier)

	// 4. Perform patch diff
	updatesMap = diffAPIs(config, updatesMap, createsMap)

	// 5. Execute creates, updates and deletes
	createsSlice := apisMapToSlice(createsMap)
	updatesSlice := apisMapToSlice(updatesMap)
    deletesSlice := apisMapToSlice(deletesMap)

	err = processAPIOperations(cli, createsSlice, updatesSlice, deletesSlice)
	if err != nil {
		return nil, err
	}

	result := auth0.ImportChanges{
		Resource: "APIs", 
		Creates: len(createsSlice), 
		Updates: len(updatesSlice), 
		Deletes: len(deletesSlice),
	}

	return &result, nil
}

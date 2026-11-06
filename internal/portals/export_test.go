package portals

import "context"

// The helpers below let the tests in portals_test call the steps of setup one by one. They are
// compiled only for tests.

// EnsuredID is the outcome of a step that creates or finds one resource.
type EnsuredID struct {
	ID     string
	Status Status
}

// EnsuredCredentials is the outcome of a step that creates or finds an application.
type EnsuredCredentials struct {
	ClientID     string
	ClientSecret string
	Status       Status
}

// PortalApplicationInput describes the application of a portal.
type PortalApplicationInput struct {
	Name           string
	Slug           string
	Template       Template
	TenantDomain   string
	Host           string
	IsCustomDomain bool
}

func EnsureResourceServer(ctx context.Context, store ResourceServerStore, spec ResourceServerSpec) (EnsuredID, error) {
	outcome, err := ensureResourceServer(ctx, store, spec)

	return EnsuredID{ID: outcome.value, Status: outcome.status}, err
}

func EnsurePortalClient(ctx context.Context, store ClientStore, input PortalApplicationInput) (EnsuredCredentials, error) {
	outcome, err := ensurePortalClient(ctx, store, portalApplication{
		name:           input.Name,
		slug:           input.Slug,
		template:       input.Template,
		tenantDomain:   input.TenantDomain,
		host:           input.Host,
		isCustomDomain: input.IsCustomDomain,
	})

	return EnsuredCredentials{
		ClientID:     outcome.value.clientID,
		ClientSecret: outcome.value.clientSecret,
		Status:       outcome.status,
	}, err
}

func PortalClientSettings(input PortalApplicationInput) ClientSettings {
	return portalClientSettings(portalApplication{
		name:           input.Name,
		slug:           input.Slug,
		template:       input.Template,
		tenantDomain:   input.TenantDomain,
		host:           input.Host,
		isCustomDomain: input.IsCustomDomain,
	})
}

func EnsureClientGrant(ctx context.Context, store ClientGrantStore, spec ClientGrantSpec) (EnsuredID, error) {
	outcome, err := ensureClientGrant(ctx, store, spec)

	return EnsuredID{ID: outcome.value, Status: outcome.status}, err
}

func EnsureVaultConnection(ctx context.Context, store VaultStore, name string, setup VaultSetup) (EnsuredID, error) {
	outcome, err := ensureVaultConnection(ctx, store, name, setup)

	return EnsuredID{ID: outcome.value, Status: outcome.status}, err
}

func RefreshVaultConnection(ctx context.Context, store VaultStore, id string, setup VaultSetup) error {
	return refreshVaultConnection(ctx, store, id, setup)
}

// EnsuredForm is the outcome of a step that creates or finds one form.
type EnsuredForm struct {
	Key    FormKey
	ID     string
	FlowID string
	Status Status
}

func EnsureForms(ctx context.Context, store FormStore, vaultConnectionID string) ([]EnsuredForm, error) {
	definitions, err := buildForms(vaultConnectionID)
	if err != nil {
		return nil, err
	}

	outcomes, err := ensureForms(ctx, store, definitions)

	forms := make([]EnsuredForm, 0, len(outcomes))
	for _, outcome := range outcomes {
		forms = append(forms, EnsuredForm{Key: outcome.key, ID: outcome.ref.id, FlowID: outcome.ref.flowID, Status: outcome.status})
	}

	return forms, err
}

func BuildFormDocuments(vaultConnectionID string) (map[FormKey]map[string]any, error) {
	definitions, err := buildForms(vaultConnectionID)
	if err != nil {
		return nil, err
	}

	documents := make(map[FormKey]map[string]any, len(definitions))
	for _, definition := range definitions {
		documents[definition.key] = definition.document
	}

	return documents, nil
}

// FormIDs are the IDs of the forms a template uses.
type FormIDs = formIDs

// Template is the pages of a kind of portal.
type PortalTemplate = portalTemplate

func TemplateFor(template Template, forms FormIDs) (PortalTemplate, error) {
	return templateFor(template, forms)
}

// EnsurePortal creates the portal of a template for the application, or finds it.
func EnsurePortal(
	ctx context.Context,
	store PortalStore,
	template PortalTemplate,
	slug, name, clientID, clientSecret string,
) (EnsuredID, error) {
	outcome, err := ensurePortal(ctx, store, portalRequest(template, slug, name, credentials{clientID: clientID, clientSecret: clientSecret}))

	return EnsuredID{ID: outcome.value.ID, Status: outcome.status}, err
}

func FormsVaultName() string { return formsVaultName }

func PortalURL(host, slug, organizationID string) string {
	return portalURL(host, slug, organizationID)
}

func AudiencesFor(template Template, tenantDomain string) map[string][]string {
	audiences := map[string][]string{}
	for _, access := range audiencesFor(template, tenantDomain) {
		audiences[access.audience] = access.scope
	}

	return audiences
}

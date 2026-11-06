package portals

import (
	"context"
	"fmt"
	"net/http"
	"slices"
)

// Every ensure function looks for its resource by a stable key before it creates one, so rerunning
// setup after a failure reuses what already exists.

// Status says what setup did with a resource.
type Status string

const (
	// StatusCreated means the resource did not exist and was created.
	StatusCreated Status = "created"
	// StatusReused means the resource already existed and was left as it is.
	StatusReused Status = "reused"
	// StatusUpdated means the resource existed and was changed.
	StatusUpdated Status = "updated"
)

// describe is the status in what setup shows.
func (s Status) describe() string {
	switch s {
	case StatusReused:
		return "already exists"
	case StatusUpdated:
		return "updated"
	default:
		return "created"
	}
}

// ensured is a resource and what was done with it.
type ensured[T any] struct {
	value  T
	status Status
}

// credentials are the ID and the secret of an application.
type credentials struct {
	clientID     string
	clientSecret string
}

func ensureResourceServer(ctx context.Context, store ResourceServerStore, spec ResourceServerSpec) (ensured[string], error) {
	existing, err := findResourceServer(ctx, store, spec.Identifier)
	if err != nil {
		return ensured[string]{}, err
	}

	if existing != nil {
		return ensured[string]{value: existing.ID, status: StatusReused}, nil
	}

	created, err := store.CreateResourceServer(ctx, spec)
	if IsStatus(err, http.StatusConflict) {
		// Another run created it between the read and the create.
		if raced, findErr := findResourceServer(ctx, store, spec.Identifier); findErr == nil && raced != nil {
			return ensured[string]{value: raced.ID, status: StatusReused}, nil
		}
	}

	if err != nil {
		return ensured[string]{}, fmt.Errorf("create resource server %q: %w", spec.Identifier, err)
	}

	return ensured[string]{value: created.ID, status: StatusCreated}, nil
}

func findResourceServer(ctx context.Context, store ResourceServerStore, identifier string) (*ResourceServer, error) {
	servers, err := store.ListResourceServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list resource servers: %w", err)
	}

	for _, server := range servers {
		if server.Identifier == identifier {
			return &server, nil
		}
	}

	return nil, nil
}

func findClientByName(ctx context.Context, store ClientStore, name string) (*Client, error) {
	clients, err := store.ListClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}

	for _, client := range clients {
		if client.Name == name {
			return &client, nil
		}
	}

	return nil, nil
}

// readClientSecret reads the secret of an application, which only the read of a single application
// returns, and only to a session with read:client_keys.
func readClientSecret(ctx context.Context, store ClientStore, clientID string) (string, error) {
	secret, err := store.ReadClientSecret(ctx, clientID)
	if err != nil {
		return "", fmt.Errorf("read the secret of application %q: %w", clientID, err)
	}

	if secret == "" {
		return "", &ClientSecretUnreadableError{ClientID: clientID}
	}

	return secret, nil
}

func ensurePortalClient(ctx context.Context, store ClientStore, app portalApplication) (ensured[credentials], error) {
	existing, err := findClientByName(ctx, store, app.name)
	if err != nil {
		return ensured[credentials]{}, err
	}

	settings := portalClientSettings(app)

	if existing == nil {
		created, createErr := store.CreateClient(ctx, settings)
		if createErr != nil {
			return ensured[credentials]{}, fmt.Errorf("create application %q: %w", app.name, createErr)
		}

		return ensured[credentials]{
			value:  credentials{clientID: created.ID, clientSecret: created.Secret},
			status: StatusCreated,
		}, nil
	}

	if existing.Metadata[portalSlugMetadata] != app.slug {
		return ensured[credentials]{}, &NameInUseError{Name: app.name}
	}

	update := ClientSettings{
		Callbacks:                   settings.Callbacks,
		AllowedLogoutURLs:           settings.AllowedLogoutURLs,
		GrantTypes:                  settings.GrantTypes,
		OrganizationUsage:           settings.OrganizationUsage,
		OrganizationRequireBehavior: settings.OrganizationRequireBehavior,
		IPHeaderTrusted:             settings.IPHeaderTrusted,
		RefreshToken:                settings.RefreshToken,
		OIDCLogout:                  settings.OIDCLogout,
	}

	// Session transfer is set only when it is missing, so a setting changed on purpose survives a rerun.
	if existing.SessionTransfer == nil || len(existing.SessionTransfer.AllowedAuthenticationMethods) == 0 {
		update.SessionTransfer = settings.SessionTransfer
	}

	if err := store.UpdateClient(ctx, existing.ID, update); err != nil {
		return ensured[credentials]{}, fmt.Errorf("update application %q: %w", app.name, err)
	}

	secret, err := readClientSecret(ctx, store, existing.ID)
	if err != nil {
		return ensured[credentials]{}, err
	}

	return ensured[credentials]{value: credentials{clientID: existing.ID, clientSecret: secret}, status: StatusReused}, nil
}

func ensureFormsVaultClient(ctx context.Context, store ClientStore, name string) (ensured[credentials], error) {
	existing, err := findClientByName(ctx, store, name)
	if err != nil {
		return ensured[credentials]{}, err
	}

	if existing != nil {
		if existing.Metadata[formsVaultMetadata] != formsVaultRole {
			return ensured[credentials]{}, &NameInUseError{Name: name}
		}

		secret, secretErr := readClientSecret(ctx, store, existing.ID)
		if secretErr != nil {
			return ensured[credentials]{}, secretErr
		}

		return ensured[credentials]{value: credentials{clientID: existing.ID, clientSecret: secret}, status: StatusReused}, nil
	}

	created, err := store.CreateClient(ctx, formsVaultClientSettings(name))
	if err != nil {
		return ensured[credentials]{}, fmt.Errorf("create application %q: %w", name, err)
	}

	return ensured[credentials]{
		value:  credentials{clientID: created.ID, clientSecret: created.Secret},
		status: StatusCreated,
	}, nil
}

func ensureClientGrant(ctx context.Context, store ClientGrantStore, spec ClientGrantSpec) (ensured[string], error) {
	grants, err := store.ListClientGrants(ctx, spec.ClientID)
	if err != nil {
		return ensured[string]{}, fmt.Errorf("list the grants of application %q: %w", spec.ClientID, err)
	}

	index := slices.IndexFunc(grants, func(grant ClientGrant) bool { return grant.Audience == spec.Audience })
	if index < 0 {
		created, createErr := store.CreateClientGrant(ctx, spec)
		if createErr != nil {
			return ensured[string]{}, fmt.Errorf("create the grant for %q: %w", spec.Audience, createErr)
		}

		return ensured[string]{value: created.ID, status: StatusCreated}, nil
	}

	existing := grants[index]
	if sameScopes(existing.Scope, spec.Scope) {
		return ensured[string]{value: existing.ID, status: StatusReused}, nil
	}

	if err := store.UpdateClientGrantScope(ctx, existing.ID, spec.Scope); err != nil {
		return ensured[string]{}, fmt.Errorf("update the grant for %q: %w", spec.Audience, err)
	}

	return ensured[string]{value: existing.ID, status: StatusUpdated}, nil
}

// sameScopes reports whether two lists hold the same scopes, whatever their order.
func sameScopes(left, right []string) bool {
	normalize := func(scopes []string) []string {
		sorted := slices.Clone(scopes)
		slices.Sort(sorted)

		return slices.Compact(sorted)
	}

	return slices.Equal(normalize(left), normalize(right))
}

func ensureVaultConnection(ctx context.Context, store VaultStore, name string, setup VaultSetup) (ensured[string], error) {
	connections, err := store.ListVaultConnections(ctx)
	if err != nil {
		return ensured[string]{}, fmt.Errorf("list the vault connections: %w", err)
	}

	for _, connection := range connections {
		if connection.Name == name {
			return ensured[string]{value: connection.ID, status: StatusReused}, nil
		}
	}

	created, err := store.CreateVaultConnection(ctx, name, setup)
	if err != nil {
		return ensured[string]{}, fmt.Errorf("create the vault connection %q: %w", name, err)
	}

	return ensured[string]{value: created.ID, status: StatusCreated}, nil
}

// refreshVaultConnection sends the setup of a vault connection again. An update without a setup
// keeps the old session, and sending it makes the vault sign in again with the grants in force.
func refreshVaultConnection(ctx context.Context, store VaultStore, id string, setup VaultSetup) error {
	if err := store.UpdateVaultConnection(ctx, id, setup); err != nil {
		return fmt.Errorf("refresh the vault connection %q: %w", id, err)
	}

	return nil
}

// formRef is a form in the tenant, with the flow it runs.
type formRef struct {
	id     string
	flowID string
}

// formOutcome is a form and what was done with it.
type formOutcome struct {
	key    FormKey
	ref    formRef
	status Status
}

func ensureForms(ctx context.Context, store FormStore, definitions []formDefinition) ([]formOutcome, error) {
	existing, err := store.ListForms(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the forms: %w", err)
	}

	outcomes := make([]formOutcome, 0, len(definitions))

	for _, definition := range definitions {
		index := slices.IndexFunc(existing, func(form Form) bool { return form.Name == definition.name })
		if index >= 0 {
			detail, readErr := store.ReadForm(ctx, existing[index].ID)
			if readErr != nil {
				return nil, fmt.Errorf("read the form %q: %w", definition.name, readErr)
			}

			outcomes = append(outcomes, formOutcome{
				key:    definition.key,
				ref:    formRef{id: existing[index].ID, flowID: detail.FlowID},
				status: StatusReused,
			})

			continue
		}

		imported, importErr := store.ImportForm(ctx, definition.document)
		if importErr != nil {
			return nil, fmt.Errorf("import the form %q: %w", definition.name, importErr)
		}

		outcomes = append(outcomes, formOutcome{
			key:    definition.key,
			ref:    formRef{id: imported.ID, flowID: imported.FlowID},
			status: StatusCreated,
		})
	}

	return outcomes, nil
}

// findPortalBySlug returns the portal with the slug, read in full with its application, or nil when
// the slug is free.
func findPortalBySlug(ctx context.Context, store PortalStore, slug string) (*Portal, error) {
	portalList, err := store.ListPortals(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the portals: %w", err)
	}

	index := slices.IndexFunc(portalList, func(portal PortalSummary) bool { return portal.Slug == slug })
	if index < 0 {
		return nil, nil
	}

	portal, err := store.ReadPortal(ctx, portalList[index].ID)
	if err != nil {
		return nil, fmt.Errorf("read portal %q: %w", slug, err)
	}

	return &portal, nil
}

func ensurePortal(ctx context.Context, store PortalStore, request CreatePortalRequest) (ensured[Portal], error) {
	existing, err := findPortalBySlug(ctx, store, request.Slug)
	if err != nil {
		return ensured[Portal]{}, err
	}

	if existing != nil {
		if existing.Client == nil || existing.Client.ClientID != request.Client.ClientID {
			return ensured[Portal]{}, &SlugTakenError{Slug: request.Slug}
		}

		return ensured[Portal]{value: *existing, status: StatusReused}, nil
	}

	created, err := store.CreatePortal(ctx, request)
	if IsStatus(err, http.StatusConflict) {
		return ensured[Portal]{}, &SlugTakenError{Slug: request.Slug}
	}

	if err != nil {
		return ensured[Portal]{}, fmt.Errorf("create the portal %q: %w", request.Slug, err)
	}

	return ensured[Portal]{value: created, status: StatusCreated}, nil
}

// portalRequest builds the request that creates a portal from its pages.
func portalRequest(template portalTemplate, slug, name string, app credentials) CreatePortalRequest {
	return CreatePortalRequest{
		Slug: slug,
		Name: name,
		Client: &PortalClient{
			TokenEndpointAuthMethod: "client_secret_post",
			ClientID:                app.clientID,
			ClientSecret:            app.clientSecret,
		},
		Navigation: &Navigation{Sidebar: &Sidebar{Components: template.Sidebar}},
		Pages:      &Pages{Default: template.DefaultPage, Standard: template.Pages},
	}
}

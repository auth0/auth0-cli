package cli

import (
	"context"
	"fmt"

	goauth0 "github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/portals"
)

// ListResourceServers implements portals.ResourceServerStore.
func (t *portalsTenant) ListResourceServers(ctx context.Context) ([]portals.ResourceServer, error) {
	servers, err := listPages(ctx, func(opts ...management.RequestOption) ([]*management.ResourceServer, bool, error) {
		list, err := t.api.ResourceServer.List(ctx, opts...)
		if err != nil {
			return nil, false, err
		}

		return list.ResourceServers, list.HasNext(), nil
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.ResourceServer, 0, len(servers))
	for _, server := range servers {
		listed = append(listed, portals.ResourceServer{
			ID:         server.GetID(),
			Identifier: server.GetIdentifier(),
			Name:       server.GetName(),
		})
	}

	return listed, nil
}

// CreateResourceServer implements portals.ResourceServerStore.
func (t *portalsTenant) CreateResourceServer(ctx context.Context, spec portals.ResourceServerSpec) (portals.ResourceServer, error) {
	server := &management.ResourceServer{
		Name:       goauth0.String(spec.Name),
		Identifier: goauth0.String(spec.Identifier),
		SkipConsentForVerifiableFirstPartyClients: goauth0.Bool(spec.SkipConsentForVerifiableFirstPartyClients),
	}
	if spec.TokenDialect != "" {
		server.TokenDialect = goauth0.String(spec.TokenDialect)
	}

	if err := t.api.ResourceServer.Create(ctx, server); err != nil {
		return portals.ResourceServer{}, asPortalsError(err)
	}

	return portals.ResourceServer{ID: server.GetID(), Identifier: server.GetIdentifier(), Name: server.GetName()}, nil
}

// ListClients implements portals.ClientStore.
func (t *portalsTenant) ListClients(ctx context.Context) ([]portals.Client, error) {
	clients, err := listPages(
		ctx,
		func(opts ...management.RequestOption) ([]*management.Client, bool, error) {
			list, err := t.api.Client.List(ctx, opts...)
			if err != nil {
				return nil, false, err
			}

			return list.Clients, list.HasNext(), nil
		},
		management.IncludeFields("client_id", "name", "client_metadata", "session_transfer"),
	)
	if err != nil {
		return nil, err
	}

	listed := make([]portals.Client, 0, len(clients))
	for _, client := range clients {
		listed = append(listed, clientFromSDK(client))
	}

	return listed, nil
}

// ReadClient implements portals.ClientStore.
func (t *portalsTenant) ReadClient(ctx context.Context, id string) (portals.Client, error) {
	client, err := t.api.Client.Read(ctx, id)
	if err != nil {
		return portals.Client{}, asPortalsError(err)
	}

	read := clientFromSDK(client)
	read.Secret = ""

	return read, nil
}

// ReadClientSecret implements portals.ClientStore. The secret is empty when the login cannot read it.
func (t *portalsTenant) ReadClientSecret(ctx context.Context, id string) (string, error) {
	client, err := t.api.Client.Read(ctx, id, management.IncludeFields("client_id", "client_secret"))
	if err != nil {
		return "", asPortalsError(err)
	}

	return client.GetClientSecret(), nil
}

// CreateClient implements portals.ClientStore.
func (t *portalsTenant) CreateClient(ctx context.Context, settings portals.ClientSettings) (portals.Client, error) {
	client := clientToSDK(settings)
	if err := t.api.Client.Create(ctx, client); err != nil {
		return portals.Client{}, asPortalsError(err)
	}

	return clientFromSDK(client), nil
}

// UpdateClient implements portals.ClientStore.
func (t *portalsTenant) UpdateClient(ctx context.Context, id string, settings portals.ClientSettings) error {
	return asPortalsError(t.api.Client.Update(ctx, id, clientToSDK(settings)))
}

// clientToSDK maps the settings that are set. A setting left empty or nil is not sent.
func clientToSDK(settings portals.ClientSettings) *management.Client {
	client := &management.Client{
		IsFirstParty:                   settings.IsFirstParty,
		OIDCConformant:                 settings.OIDCConformant,
		IsTokenEndpointIPHeaderTrusted: settings.IPHeaderTrusted,
		RequireProofOfPossession:       settings.RequireProofOfPossession,
	}

	setString(&client.Name, settings.Name)
	setString(&client.AppType, settings.AppType)
	setString(&client.TokenEndpointAuthMethod, settings.TokenEndpointAuthMethod)
	setString(&client.OrganizationUsage, settings.OrganizationUsage)
	setString(&client.OrganizationRequireBehavior, settings.OrganizationRequireBehavior)
	setStrings(&client.Callbacks, settings.Callbacks)
	setStrings(&client.AllowedLogoutURLs, settings.AllowedLogoutURLs)
	setStrings(&client.GrantTypes, settings.GrantTypes)

	if settings.Metadata != nil {
		metadata := make(map[string]interface{}, len(settings.Metadata))
		for key, value := range settings.Metadata {
			metadata[key] = value
		}
		client.ClientMetadata = &metadata
	}

	if token := settings.RefreshToken; token != nil {
		client.RefreshToken = refreshTokenToSDK(token)
	}

	if transfer := settings.SessionTransfer; transfer != nil {
		client.SessionTransfer = &management.SessionTransfer{}
		setStrings(&client.SessionTransfer.AllowedAuthenticationMethods, transfer.AllowedAuthenticationMethods)
		setString(&client.SessionTransfer.EnforceDeviceBinding, transfer.EnforceDeviceBinding)
	}

	if logout := settings.OIDCLogout; logout != nil {
		client.OIDCLogout = &management.OIDCLogout{
			BackChannelLogoutInitiators: &management.BackChannelLogoutInitiators{},
		}
		setStrings(&client.OIDCLogout.BackChannelLogoutURLs, logout.BackchannelLogoutURLs)
		setString(&client.OIDCLogout.BackChannelLogoutInitiators.Mode, logout.Mode)
		setStrings(&client.OIDCLogout.BackChannelLogoutInitiators.SelectedInitiators, logout.SelectedInitiators)
	}

	return client
}

func refreshTokenToSDK(token *portals.RefreshToken) *management.ClientRefreshToken {
	refresh := &management.ClientRefreshToken{
		TokenLifetime:             token.TokenLifetime,
		IdleTokenLifetime:         token.IdleTokenLifetime,
		InfiniteIdleTokenLifetime: token.InfiniteIdleTokenLifetime,
	}
	setString(&refresh.RotationType, token.RotationType)
	setString(&refresh.ExpirationType, token.ExpirationType)

	if len(token.Policies) > 0 {
		policies := make([]management.ClientRefreshTokenPolicy, 0, len(token.Policies))
		for _, policy := range token.Policies {
			entry := management.ClientRefreshTokenPolicy{Audience: goauth0.String(policy.Audience)}
			setStrings(&entry.Scope, policy.Scope)
			policies = append(policies, entry)
		}
		refresh.Policies = &policies
	}

	return refresh
}

func setString(target **string, value string) {
	if value != "" {
		*target = goauth0.String(value)
	}
}

func setStrings(target **[]string, values []string) {
	if len(values) > 0 {
		copied := append([]string(nil), values...)
		*target = &copied
	}
}

// clientFromSDK maps the parts of an application the portals package reads.
func clientFromSDK(client *management.Client) portals.Client {
	settings := portals.ClientSettings{
		Name:                        client.GetName(),
		AppType:                     client.GetAppType(),
		IsFirstParty:                client.IsFirstParty,
		OIDCConformant:              client.OIDCConformant,
		TokenEndpointAuthMethod:     client.GetTokenEndpointAuthMethod(),
		IPHeaderTrusted:             client.IsTokenEndpointIPHeaderTrusted,
		RequireProofOfPossession:    client.RequireProofOfPossession,
		Callbacks:                   client.GetCallbacks(),
		AllowedLogoutURLs:           client.GetAllowedLogoutURLs(),
		GrantTypes:                  client.GetGrantTypes(),
		OrganizationUsage:           client.GetOrganizationUsage(),
		OrganizationRequireBehavior: client.GetOrganizationRequireBehavior(),
		Metadata:                    metadataFromSDK(client.ClientMetadata),
	}

	if token := client.RefreshToken; token != nil {
		settings.RefreshToken = &portals.RefreshToken{
			RotationType:              token.GetRotationType(),
			ExpirationType:            token.GetExpirationType(),
			TokenLifetime:             token.TokenLifetime,
			IdleTokenLifetime:         token.IdleTokenLifetime,
			InfiniteIdleTokenLifetime: token.InfiniteIdleTokenLifetime,
		}

		if token.Policies != nil {
			for _, policy := range *token.Policies {
				settings.RefreshToken.Policies = append(settings.RefreshToken.Policies, portals.RefreshTokenPolicy{
					Audience: policy.GetAudience(),
					Scope:    policy.GetScope(),
				})
			}
		}
	}

	if transfer := client.SessionTransfer; transfer != nil {
		settings.SessionTransfer = &portals.SessionTransfer{
			AllowedAuthenticationMethods: transfer.GetAllowedAuthenticationMethods(),
			EnforceDeviceBinding:         transfer.GetEnforceDeviceBinding(),
		}
	}

	if logout := client.OIDCLogout; logout != nil {
		settings.OIDCLogout = &portals.OIDCLogout{BackchannelLogoutURLs: logout.GetBackChannelLogoutURLs()}

		if initiators := logout.BackChannelLogoutInitiators; initiators != nil {
			settings.OIDCLogout.Mode = initiators.GetMode()
			settings.OIDCLogout.SelectedInitiators = initiators.GetSelectedInitiators()
		}
	}

	return portals.Client{
		ID:             client.GetClientID(),
		Secret:         client.GetClientSecret(),
		ClientSettings: settings,
	}
}

// metadataFromSDK keeps the text values of the metadata, which is all the portals package sets.
func metadataFromSDK(metadata *map[string]interface{}) map[string]string {
	if metadata == nil {
		return nil
	}

	text := make(map[string]string, len(*metadata))
	for key, value := range *metadata {
		if asString, ok := value.(string); ok {
			text[key] = asString
		}
	}

	return text
}

// ListClientGrants implements portals.ClientGrantStore.
func (t *portalsTenant) ListClientGrants(ctx context.Context, clientID string) ([]portals.ClientGrant, error) {
	grants, err := pageAll(ctx, func(ctx context.Context) (*auth0ClientGrantPage, error) {
		return t.apiv3.ClientGrant.List(ctx, &managementv3.ListClientGrantsRequestParameters{
			ClientID: goauth0.String(clientID),
			Take:     goauth0.Int(portalsPageSize),
		})
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.ClientGrant, 0, len(grants))
	for _, grant := range grants {
		listed = append(listed, portals.ClientGrant{
			ID:       grant.GetID(),
			ClientID: grant.GetClientID(),
			Audience: grant.GetAudience(),
			Scope:    grant.GetScope(),
		})
	}

	return listed, nil
}

// CreateClientGrant implements portals.ClientGrantStore.
func (t *portalsTenant) CreateClientGrant(ctx context.Context, spec portals.ClientGrantSpec) (portals.ClientGrant, error) {
	request := &managementv3.CreateClientGrantRequestContent{
		ClientID: goauth0.String(spec.ClientID),
		Audience: spec.Audience,
		Scope:    spec.Scope,
	}

	switch spec.SubjectType {
	case "":
	case "user":
		subject := managementv3.ClientGrantSubjectTypeEnumUser
		request.SubjectType = &subject
	case "client":
		subject := managementv3.ClientGrantSubjectTypeEnumClient
		request.SubjectType = &subject
	default:
		return portals.ClientGrant{}, fmt.Errorf("unknown client grant subject type %q", spec.SubjectType)
	}

	created, err := t.apiv3.ClientGrant.Create(ctx, request)
	if err != nil {
		return portals.ClientGrant{}, asPortalsError(err)
	}

	return portals.ClientGrant{
		ID:       created.GetID(),
		ClientID: created.GetClientID(),
		Audience: created.GetAudience(),
		Scope:    created.GetScope(),
	}, nil
}

// UpdateClientGrantScope implements portals.ClientGrantStore.
func (t *portalsTenant) UpdateClientGrantScope(ctx context.Context, id string, scope []string) error {
	_, err := t.apiv3.ClientGrant.Update(ctx, id, &managementv3.UpdateClientGrantRequestContent{Scope: scope})

	return asPortalsError(err)
}

// ListVaultConnections implements portals.VaultStore.
func (t *portalsTenant) ListVaultConnections(ctx context.Context) ([]portals.VaultConnection, error) {
	list, err := t.api.FlowVaultConnection.GetConnectionList(ctx, management.Page(0), management.PerPage(portalsPageSize))
	if err != nil {
		return nil, asPortalsError(err)
	}

	listed := make([]portals.VaultConnection, 0, len(list.Connections))
	for _, connection := range list.Connections {
		listed = append(listed, portals.VaultConnection{ID: connection.GetID(), Name: connection.GetName()})
	}

	return listed, nil
}

// CreateVaultConnection implements portals.VaultStore.
func (t *portalsTenant) CreateVaultConnection(ctx context.Context, name string, setup portals.VaultSetup) (portals.VaultConnection, error) {
	connection := &management.FlowVaultConnection{
		AppID: goauth0.String("AUTH0"),
		Name:  goauth0.String(name),
		Setup: vaultSetupToSDK(setup),
	}

	if err := t.api.FlowVaultConnection.CreateConnection(ctx, connection); err != nil {
		return portals.VaultConnection{}, asPortalsError(err)
	}

	return portals.VaultConnection{ID: connection.GetID(), Name: connection.GetName()}, nil
}

// UpdateVaultConnection implements portals.VaultStore.
func (t *portalsTenant) UpdateVaultConnection(ctx context.Context, id string, setup portals.VaultSetup) error {
	return asPortalsError(t.api.FlowVaultConnection.UpdateConnection(ctx, id, &management.FlowVaultConnection{
		Setup: vaultSetupToSDK(setup),
	}))
}

func vaultSetupToSDK(setup portals.VaultSetup) *map[string]interface{} {
	return &map[string]interface{}{
		"domain":        setup.Domain,
		"client_id":     setup.ClientID,
		"client_secret": setup.ClientSecret,
		"type":          "OAUTH_APP",
	}
}

// ListDatabaseConnections implements portals.ConnectionStore.
func (t *portalsTenant) ListDatabaseConnections(ctx context.Context) ([]portals.Connection, error) {
	strategy := managementv3.ConnectionStrategyEnumAuth0

	connections, err := pageAll(ctx, func(ctx context.Context) (*auth0ConnectionPage, error) {
		return t.apiv3.Connection.List(ctx, &managementv3.ListConnectionsQueryParameters{
			Strategy: []*managementv3.ConnectionStrategyEnum{&strategy},
			Take:     goauth0.Int(portalsPageSize),
		})
	})
	if err != nil {
		return nil, err
	}

	return connectionsFromSDK(connections), nil
}

// ListClientConnections implements portals.ConnectionStore.
func (t *portalsTenant) ListClientConnections(ctx context.Context, clientID string) ([]portals.Connection, error) {
	connections, err := pageAll(ctx, func(ctx context.Context) (*auth0ClientConnectionPage, error) {
		// The endpoint pages by checkpoint and rejects page and per_page. The request must not be nil.
		return t.apiv3.ClientConnection.Get(ctx, clientID, &managementv3.ConnectionsGetRequest{
			Take: goauth0.Int(portalsPageSize),
		})
	})
	if err != nil {
		return nil, err
	}

	return connectionsFromSDK(connections), nil
}

func connectionsFromSDK(connections []*managementv3.ConnectionForList) []portals.Connection {
	listed := make([]portals.Connection, 0, len(connections))
	for _, connection := range connections {
		listed = append(listed, portals.Connection{ID: connection.GetID(), Name: connection.GetName()})
	}

	return listed
}

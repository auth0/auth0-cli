package cli

import (
	"context"
	"regexp"

	"github.com/auth0/go-auth0/management"
	"github.com/google/uuid"

	"github.com/auth0/auth0-cli/internal/auth0"
)

var defaultResources = []string{"auth0_action", "auth0_attack_protection", "auth0_branding", "auth0_client", "auth0_client_grant", "auth0_connection", "auth0_custom_domain", "auth0_email_provider", "auth0_log_stream", "auth0_organization", "auth0_pages", "auth0_resource_server", "auth0_role", "auth0_tenant"}

type (
	importDataList []importDataItem

	importDataItem struct {
		ResourceName string
		ImportID     string
	}

	resourceDataFetcher interface {
		FetchData(ctx context.Context) (importDataList, error)
	}
)

type (
	actionResourceFetcher struct {
		api *auth0.API
	}

	attackProtectionResourceFetcher struct{}

	brandingResourceFetcher struct{}
	clientResourceFetcher   struct {
		api *auth0.API
	}

	clientGrantResourceFetcher struct {
		api *auth0.API
	}

	connectionResourceFetcher struct {
		api *auth0.API
	}

	customDomainResourceFetcher struct {
		api *auth0.API
	}

	emailProviderResourceFetcher struct{}

	logStreamResourceFetcher struct {
		api *auth0.API
	}
	organizationResourceFetcher struct {
		api *auth0.API
	}

	pagesResourceFetcher          struct{}
	resourceServerResourceFetcher struct {
		api *auth0.API
	}

	roleResourceFetcher struct {
		api *auth0.API
	}

	tenantResourceFetcher struct{}
)

func (f *attackProtectionResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_attack_protection.attack_protection",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *brandingResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_branding.branding",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *clientResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		clients, err := f.api.Client.List(
			ctx,
			management.Page(page),
			management.Parameter("is_global", "false"),
			management.IncludeFields("client_id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, client := range clients.Clients {
			data = append(data, importDataItem{
				ResourceName: "auth0_client." + sanitizeResourceName(client.GetName()),
				ImportID:     client.GetClientID(),
			})
		}

		if !clients.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *clientGrantResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		grants, err := f.api.ClientGrant.List(
			ctx,
			management.Page(page),
		)
		if err != nil {
			return nil, err
		}

		for _, grant := range grants.ClientGrants {
			data = append(data, importDataItem{
				ResourceName: "auth0_client_grant." + grant.GetClientID() + "_" + sanitizeResourceName(grant.GetAudience()),
				ImportID:     grant.GetID(),
			})
		}

		if !grants.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *connectionResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		connections, err := f.api.Connection.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, connection := range connections.Connections {
			data = append(data,
				importDataItem{
					ResourceName: "auth0_connection." + sanitizeResourceName(connection.GetName()),
					ImportID:     connection.GetID(),
				},
				importDataItem{
					ResourceName: "auth0_connection_clients." + sanitizeResourceName(connection.GetName()),
					ImportID:     connection.GetID(),
				},
			)
		}

		if !connections.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *customDomainResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	customDomains, err := f.api.CustomDomain.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, domain := range customDomains {
		data = append(data, importDataItem{
			ResourceName: "auth0_custom_domain." + sanitizeResourceName(domain.GetDomain()),
			ImportID:     domain.GetID(),
		})
	}

	return data, nil
}

func (f *emailProviderResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_email_provider.email_provider",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *logStreamResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	logStreams, err := f.api.LogStream.List(ctx)
	if err != nil {
		return data, err
	}

	for _, log := range logStreams {
		data = append(data, importDataItem{
			ResourceName: "auth0_log_stream." + sanitizeResourceName(log.GetName()),
			ImportID:     log.GetID(),
		})
	}

	return data, nil
}

func (f *organizationResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	orgs, err := getWithPagination(
		100,
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
			res, err := f.api.Organization.List(ctx, opts...)
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
		return data, err
	}

	for _, org := range orgs {
		data = append(data, importDataItem{
			ResourceName: "auth0_organization." + sanitizeResourceName(org.(*management.Organization).GetName()),
			ImportID:     org.(*management.Organization).GetID(),
		})
	}

	return data, nil
}

func (f *pagesResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_pages.pages",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *resourceServerResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		apis, err := f.api.ResourceServer.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name"),
			management.PerPage(100),
		)
		if err != nil {
			return nil, err
		}

		for _, api := range apis.ResourceServers {
			data = append(data, importDataItem{
				ResourceName: "auth0_resource_server." + sanitizeResourceName(api.GetName()),
				ImportID:     api.GetID(),
			})
			data = append(data, importDataItem{
				ResourceName: "auth0_resource_server_scopes." + sanitizeResourceName(api.GetName()),
				ImportID:     api.GetID(),
			})
		}

		if !apis.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *roleResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		roles, err := f.api.Role.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, role := range roles.Roles {
			data = append(data,
				importDataItem{
					ResourceName: "auth0_role." + sanitizeResourceName(role.GetName()),
					ImportID:     role.GetID(),
				},
			)

			rolePerms, err := f.api.Role.Permissions(ctx, role.GetID())
			if err != nil {
				return data, nil
			}
			if len(rolePerms.Permissions) > 0 {
				// `permissions` block a required field for TF Provider; cannot have empty permissions
				data = append(data, importDataItem{
					ResourceName: "auth0_role_permissions." + sanitizeResourceName(role.GetName()),
					ImportID:     role.GetID(),
				})
			}
		}

		if !roles.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *tenantResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_tenant.tenant",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *actionResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		actions, err := f.api.Action.List(
			ctx,
			management.Page(page),
		)
		if err != nil {
			return nil, err
		}

		for _, action := range actions.Actions {
			data = append(data, importDataItem{
				ResourceName: "auth0_action." + sanitizeResourceName(action.GetName()),
				ImportID:     action.GetID(),
			})
		}

		if !actions.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

// sanitizeResourceName will return a valid terraform resource name.
//
// A name must start with a letter or underscore and may
// contain only letters, digits, underscores, and dashes.
func sanitizeResourceName(name string) string {
	// Regular expression pattern to remove invalid characters.
	namePattern := "[^a-zA-Z0-9_-]+"
	re := regexp.MustCompile(namePattern)

	sanitizedName := re.ReplaceAllString(name, "")

	// Regular expression pattern to remove leading digits or dashes.
	namePattern = "^[0-9-]+"
	re = regexp.MustCompile(namePattern)

	sanitizedName = re.ReplaceAllString(sanitizedName, "")

	return sanitizedName
}

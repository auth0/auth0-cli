package cli

import (
	"context"
	"regexp"

	"github.com/auth0/go-auth0/management"
	"github.com/google/uuid"

	"github.com/auth0/auth0-cli/internal/auth0"
)

var defaultResources = []string{"auth0_action", "auth0_client", "auth0_connection", "auth0_organization", "auth0_tenant"}

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
	clientResourceFetcher struct {
		api *auth0.API
	}

	connectionResourceFetcher struct {
		api *auth0.API
	}

	organizationResourceFetcher struct {
		api *auth0.API
	}

	tenantResourceFetcher struct{}
)

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
			data = append(data, importDataItem{
				ResourceName: "auth0_connection." + sanitizeResourceName(connection.GetName()),
				ImportID:     connection.GetID(),
			})
		}

		if !connections.HasNext() {
			break
		}

		page++
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

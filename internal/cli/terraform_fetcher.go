package cli

import (
	"context"
	"regexp"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/auth0"
)

type (
	importDataList []importDataItem

	importDataItem struct {
		ResourceName string
		ImportID     string
	}

	resourceDataFetcher interface {
		FetchData(ctx context.Context) (importDataList, error)
	}

	clientResourceFetcher struct {
		api *auth0.API
	}

	connectionResourceFetcher struct {
		api *auth0.API
	}
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

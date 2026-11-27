//go:generate go tool mockgen -source=organization_v3.go -destination=mock/organization_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// OrganizationSearchPage is one cursor-paginated page of organizations search results.
// It is aliased here so the mock generator (which cannot parse instantiated generic
// types) sees a plain named type in the interface.
type OrganizationSearchPage = core.Page[*string, *managementv3.SearchOrganization, *managementv3.SearchOrganizationsPaginatedResponseContent]

// OrganizationAPIV3 is the V3 SDK interface for the eventually consistent
// organizations search endpoint (GET /organizations/search).
type OrganizationAPIV3 interface {
	// Search organizations using SCIM or Lucene filter syntax.
	//
	// Required scope: `read:organizations` or `read:organizations_summary`
	//
	// See: https://auth0.com/docs/api/management/v2/organizations/get-search
	Search(
		ctx context.Context,
		request *managementv3.SearchOrganizationsRequestParameters,
		opts ...option.RequestOption,
	) (*OrganizationSearchPage, error)
}

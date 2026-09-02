//go:generate mockgen -source=form.go -destination=mock/form_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// FormSummaryPage aliases the paginated forms list response. The alias keeps the
// interface return type a single identifier so mockgen's source parser can handle
// it (it cannot parse the multi-type-parameter generic inline).
type FormSummaryPage = core.Page[*int, *managementv3.FormSummary, *managementv3.ListFormsOffsetPaginatedResponseContent]

// FormAPIV3 is the V3 SDK interface for the /forms endpoint.
type FormAPIV3 interface {
	// List forms.
	//
	// Required scope: `read:forms`.
	List(
		ctx context.Context,
		request *managementv3.ListFormsRequestParameters,
		opts ...option.RequestOption,
	) (*FormSummaryPage, error)

	// Get retrieves a form by its ID.
	//
	// Required scope: `read:forms`.
	Get(
		ctx context.Context,
		id string,
		request *managementv3.GetFormRequestParameters,
		opts ...option.RequestOption,
	) (*managementv3.GetFormResponseContent, error)

	// Delete a form.
	//
	// Required scope: `delete:forms`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

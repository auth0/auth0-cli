package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/auth0/go-auth0/management"
	"github.com/auth0/go-auth0/v3/management/core"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/portals"
)

const (
	// The number of items asked for in each page.
	portalsPageSize = 100
	// The number of pages after which a listing that never ends is stopped.
	portalsMaxPages = 100
)

// portalsTenant implements portals.API on the typed Management API clients of the CLI. Only the
// requests no typed client can send go through the raw HTTP client.
type portalsTenant struct {
	api   *auth0.API
	apiv3 *auth0.APIV3
	// Set when a request that changes the tenant succeeded.
	wrote atomic.Bool
}

var _ portals.API = (*portalsTenant)(nil)

func newPortalsTenant(api *auth0.API, apiv3 *auth0.APIV3) *portalsTenant {
	return &portalsTenant{api: api, apiv3: apiv3}
}

// written records that a request changed the tenant, when it succeeded, and returns its error.
func (t *portalsTenant) written(err error) error {
	if err == nil {
		t.wrote.Store(true)
	}

	return err
}

// hasWritten reports whether any request that changes the tenant succeeded.
func (t *portalsTenant) hasWritten() bool {
	return t.wrote.Load()
}

// asPortalsError turns a failure of the SDK into a *portals.APIError, so the portals package reads
// the status and the error code the same way whichever client made the request. Any other error is
// returned as it is.
func asPortalsError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*portals.APIError](err); ok {
		return err
	}

	if v1, ok := errors.AsType[management.Error](err); ok {
		// The text of the error is "<status> <reason>: <message>", and the reason has no colon.
		_, message, _ := strings.Cut(v1.Error(), ": ")

		return &portals.APIError{Status: v1.Status(), Code: v1.Code(), Message: message}
	}

	if v3, ok := errors.AsType[*core.APIError](err); ok {
		_, body, _ := strings.Cut(v3.Error(), ": ")

		return apiErrorFromBody(v3.StatusCode, []byte(body))
	}

	return err
}

// apiErrorFromBody builds the error of a response from its status and body.
func apiErrorFromBody(status int, body []byte) *portals.APIError {
	apiErr := &portals.APIError{Status: status, Message: strings.TrimSpace(string(body))}

	var parsed struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && (parsed.ErrorCode != "" || parsed.Message != "") {
		apiErr.Code = parsed.ErrorCode
		apiErr.Message = parsed.Message
	}

	return apiErr
}

// listPages reads every page of a v1 listing, up to portalsMaxPages pages of portalsPageSize items.
func listPages[T any](
	ctx context.Context,
	fetch func(opts ...management.RequestOption) (items []T, hasNext bool, err error),
	extra ...management.RequestOption,
) ([]T, error) {
	var all []T

	for page := 0; page < portalsMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		opts := append([]management.RequestOption{management.Page(page), management.PerPage(portalsPageSize)}, extra...)

		items, hasNext, err := fetch(opts...)
		if err != nil {
			return nil, asPortalsError(err)
		}

		all = append(all, items...)
		if !hasNext {
			return all, nil
		}
	}

	return nil, fmt.Errorf("the listing returned more than %d items and did not stop paging", portalsMaxPages*portalsPageSize)
}

// pageAll reads every page of a v3 listing, up to portalsMaxPages pages.
func pageAll[C comparable, T any, R any](
	ctx context.Context,
	first func(ctx context.Context) (*core.Page[C, T, R], error),
) ([]T, error) {
	var all []T

	page, err := first(ctx)
	if err != nil {
		return nil, asPortalsError(err)
	}

	for number := 0; page != nil && number < portalsMaxPages; number++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		all = append(all, page.Results...)

		next, err := page.GetNextPage(ctx)
		if errors.Is(err, core.ErrNoPages) {
			return all, nil
		}

		if err != nil {
			return nil, asPortalsError(err)
		}

		page = next
	}

	if page != nil {
		return nil, fmt.Errorf("the listing returned more than %d pages and did not stop paging", portalsMaxPages)
	}

	return all, nil
}

// The pages the generic helpers read, under the names the auth0 package gives them.
type (
	auth0ClientGrantPage      = auth0.ClientGrantPage
	auth0ConnectionPage       = auth0.ConnectionForListPage
	auth0ClientConnectionPage = auth0.ClientConnectionPage
)

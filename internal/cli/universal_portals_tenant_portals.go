package cli

import (
	"context"
	"encoding/json"
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/portals"
)

// The portals are read and written with the typed Portals service of the SDK. The portals package has
// types of its own for a portal, which the audit reads as maps, and both describe the same JSON, so a
// value is moved between them as JSON.
//
// The typed model keeps the types it knows, and the fields the SDK reads and does not model
// (GetExtraProperties) are not written back, so a field the API adds later is lost on an update until the
// SDK models it.

// convertJSON moves a value to another type that describes the same JSON.
func convertJSON(from, to any) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, to)
}

// ListPortals implements portals.PortalStore. The endpoint does not page, so it is one request.
func (t *portalsTenant) ListPortals(ctx context.Context) ([]portals.PortalSummary, error) {
	list, err := t.apiv3.Portal.List(ctx)
	if err != nil {
		return nil, asPortalsError(err)
	}

	listed := make([]portals.PortalSummary, 0, len(list.Portals))
	for _, portal := range list.Portals {
		listed = append(listed, portals.PortalSummary{ID: portal.GetID(), Slug: portal.GetSlug(), Name: portal.GetName()})
	}

	return listed, nil
}

// ReadPortal implements portals.PortalStore.
func (t *portalsTenant) ReadPortal(ctx context.Context, id string) (portals.Portal, error) {
	read, err := t.apiv3.Portal.Get(ctx, id)
	if err != nil {
		return portals.Portal{}, asPortalsError(err)
	}

	var portal portals.Portal
	if err := convertJSON(read, &portal); err != nil {
		return portals.Portal{}, fmt.Errorf("read portal %q: %w", id, err)
	}

	return portal, nil
}

// CreatePortal implements portals.PortalStore.
func (t *portalsTenant) CreatePortal(ctx context.Context, request portals.CreatePortalRequest) (portals.Portal, error) {
	var content managementv3.CreatePortalRequestContent
	if err := convertJSON(request, &content); err != nil {
		return portals.Portal{}, fmt.Errorf("build the request to create portal %q: %w", request.Slug, err)
	}

	created, err := t.apiv3.Portal.Create(ctx, &content)
	if err := t.written(err); err != nil {
		return portals.Portal{}, asPortalsError(err)
	}

	var portal portals.Portal
	if err := convertJSON(created, &portal); err != nil {
		return portals.Portal{}, fmt.Errorf("read the created portal: %w", err)
	}

	return portal, nil
}

// UpdatePortal implements portals.PortalStore.
func (t *portalsTenant) UpdatePortal(ctx context.Context, id string, patch portals.PortalPatch) error {
	var content managementv3.UpdatePortalRequestContent
	if err := convertJSON(patch, &content); err != nil {
		return fmt.Errorf("build the request to update portal %q: %w", id, err)
	}

	_, err := t.apiv3.Portal.Update(ctx, id, &content)

	return asPortalsError(t.written(err))
}

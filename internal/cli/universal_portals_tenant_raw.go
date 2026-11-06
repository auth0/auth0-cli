package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	goauth0 "github.com/auth0/go-auth0"
	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/portals"
)

// portalsRequestTimeout is how long a raw request may take. The command has no deadline of its own,
// so a slow tenant cannot cancel a setup that is half way.
const portalsRequestTimeout = 30 * time.Second

// rawJSON sends a request that no typed client can send and returns the body of the response. The
// path is relative to the Management API, and may carry a query. It is for paths the CLI wrote
// itself: an id that came from the user goes through rawJSONAt.
func (t *portalsTenant) rawJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	route, query, hasQuery := strings.Cut(path, "?")
	if hasQuery {
		query = "?" + query
	}

	return t.rawJSONAt(ctx, method, strings.Split(route, "/"), query, body)
}

// rawJSONAt is rawJSON for a route given as its segments, each escaped, so an id with a "/" or a "?"
// stays one segment and cannot change the route or add a query. The query, if any, starts with "?".
func (t *portalsTenant) rawJSONAt(ctx context.Context, method string, segments []string, query string, body any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, portalsRequestTimeout)
	defer cancel()

	route := strings.Join(segments, "/")
	uri := t.api.HTTPClient.URI(segments...) + query

	request, err := t.api.HTTPClient.NewRequest(ctx, method, uri, body)
	if err != nil {
		return nil, fmt.Errorf("build the %s %s request: %w", method, route, err)
	}

	response, err := t.api.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send the %s %s request: %w", method, route, err)
	}

	defer func() {
		_ = response.Body.Close()
	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read the response to %s %s: %w", method, route, err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, apiErrorFromBody(response.StatusCode, data)
	}

	return data, nil
}

// rawList reads every page of a listing that pages with page and per_page. The listing is an array,
// or an object that holds the array under key.
func rawList[T any](ctx context.Context, t *portalsTenant, route, key string) ([]T, error) {
	var all []T

	for page := range portalsMaxPages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		path := route + "?page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(portalsPageSize)

		data, err := t.rawJSON(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		items, err := listItems[T](data, key)
		if err != nil {
			return nil, fmt.Errorf("read the listing of %s: %w", route, err)
		}

		all = append(all, items...)
		if len(items) < portalsPageSize {
			return all, nil
		}
	}

	return nil, fmt.Errorf("%s returned more than %d items and did not stop paging", route, portalsMaxPages*portalsPageSize)
}

// listItems reads a listing that is an array, or an object that holds the array under key.
func listItems[T any](data []byte, key string) ([]T, error) {
	var items []T
	if err := json.Unmarshal(data, &items); err == nil {
		return items, nil
	}

	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}

	raw, found := wrapped[key]
	if !found {
		return nil, nil
	}

	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// ListForms implements portals.FormStore.
func (t *portalsTenant) ListForms(ctx context.Context) ([]portals.Form, error) {
	summaries, err := pageAll(ctx, func(ctx context.Context) (*auth0.FormSummaryPage, error) {
		return t.apiv3.Form.List(ctx, &managementv3.ListFormsRequestParameters{PerPage: goauth0.Int(portalsPageSize)})
	})
	if err != nil {
		return nil, err
	}

	listed := make([]portals.Form, 0, len(summaries))
	for _, summary := range summaries {
		listed = append(listed, portals.Form{ID: summary.ID, Name: summary.Name})
	}

	return listed, nil
}

// formNodes is the part of a form the flow ID is read from. A flow node names the flow it runs.
type formNodes struct {
	Nodes []struct {
		Type   string `json:"type"`
		Config struct {
			FlowID string `json:"flow_id"`
		} `json:"config"`
	} `json:"nodes"`
}

func (f formNodes) flowID() string {
	for _, node := range f.Nodes {
		if node.Type == "FLOW" && node.Config.FlowID != "" {
			return node.Config.FlowID
		}
	}

	return ""
}

// ReadForm implements portals.FormStore. It reads the form raw, because a typed read fails on a
// node type the SDK does not know.
func (t *portalsTenant) ReadForm(ctx context.Context, id string) (portals.FormDetail, error) {
	data, err := t.rawJSONAt(ctx, http.MethodGet, []string{"forms", id}, "", nil)
	if err != nil {
		return portals.FormDetail{}, err
	}

	var nodes formNodes
	if err := json.Unmarshal(data, &nodes); err != nil {
		return portals.FormDetail{}, fmt.Errorf("read form %q: %w", id, err)
	}

	return portals.FormDetail{ID: id, FlowID: nodes.flowID()}, nil
}

// ImportForm implements portals.FormStore.
func (t *portalsTenant) ImportForm(ctx context.Context, document any) (portals.ImportedForm, error) {
	data, err := t.rawJSON(ctx, http.MethodPost, "forms/import", document)
	if err := t.written(err); err != nil {
		return portals.ImportedForm{}, err
	}

	var imported struct {
		Form struct {
			ID string `json:"id"`
			formNodes
		} `json:"form"`
		// The flows are an array, or an object keyed by the placeholder of the document.
		Flows json.RawMessage `json:"flows"`
	}
	if err := json.Unmarshal(data, &imported); err != nil {
		return portals.ImportedForm{}, fmt.Errorf("read the imported form: %w", err)
	}

	if imported.Form.ID == "" {
		return portals.ImportedForm{}, errors.New("the import did not return the ID of the form")
	}

	flowID := firstFlowID(imported.Flows)
	if flowID == "" {
		flowID = imported.Form.flowID()
	}

	return portals.ImportedForm{ID: imported.Form.ID, FlowID: flowID}, nil
}

// firstFlowID returns the ID of the first flow in an array or an object of flows.
func firstFlowID(flows json.RawMessage) string {
	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(flows, &list); err == nil {
		for _, flow := range list {
			if flow.ID != "" {
				return flow.ID
			}
		}

		return ""
	}

	var keyed map[string]struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(flows, &keyed); err != nil {
		return ""
	}

	// The order of an object is not defined, so the smallest key makes the choice repeatable.
	var first string
	for key, flow := range keyed {
		if flow.ID != "" && (first == "" || key < first) {
			first = key
		}
	}

	return keyed[first].ID
}

// ListPortals implements portals.PortalStore.
func (t *portalsTenant) ListPortals(ctx context.Context) ([]portals.PortalSummary, error) {
	items, err := rawList[struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	}](ctx, t, "portals", "portals")
	if err != nil {
		return nil, err
	}

	listed := make([]portals.PortalSummary, 0, len(items))
	for _, item := range items {
		listed = append(listed, portals.PortalSummary{ID: item.ID, Slug: item.Slug, Name: item.Name})
	}

	return listed, nil
}

// ReadPortal implements portals.PortalStore.
func (t *portalsTenant) ReadPortal(ctx context.Context, id string) (portals.Portal, error) {
	data, err := t.rawJSONAt(ctx, http.MethodGet, []string{"portals", id}, "", nil)
	if err != nil {
		return portals.Portal{}, err
	}

	var portal portals.Portal
	if err := json.Unmarshal(data, &portal); err != nil {
		return portals.Portal{}, fmt.Errorf("read portal %q: %w", id, err)
	}

	return portal, nil
}

// CreatePortal implements portals.PortalStore.
func (t *portalsTenant) CreatePortal(ctx context.Context, request portals.CreatePortalRequest) (portals.Portal, error) {
	data, err := t.rawJSON(ctx, http.MethodPost, "portals", request)
	if err := t.written(err); err != nil {
		return portals.Portal{}, err
	}

	var portal portals.Portal
	if err := json.Unmarshal(data, &portal); err != nil {
		return portals.Portal{}, fmt.Errorf("read the created portal: %w", err)
	}

	return portal, nil
}

// UpdatePortal implements portals.PortalStore.
func (t *portalsTenant) UpdatePortal(ctx context.Context, id string, patch portals.PortalPatch) error {
	_, err := t.rawJSONAt(ctx, http.MethodPatch, []string{"portals", id}, "", patch)

	return t.written(err)
}

// Probe implements portals.Prober.
func (t *portalsTenant) Probe(ctx context.Context, method, path string, body any) error {
	_, err := t.rawJSON(ctx, method, path, body)

	return err
}

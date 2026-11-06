package portals

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// snapshot is everything the audit needs to know about one portal, read from the tenant. The rules work
// on the snapshot only, so they can be tested without a tenant.
type snapshot struct {
	TenantDomain string
	Portal       Portal
	// Application is nil when the portal has none, or when it was deleted.
	Application *Client
	Grants      []ClientGrant
	// ResourceServers are the identifiers of the resource servers of the tenant.
	ResourceServers []string
	// Forms are the forms the portal uses, and whether each exists in the tenant.
	Forms []formStatus
	// ApplicationConnections are the connections enabled for the application.
	ApplicationConnections []Connection
	CustomDomain           *CustomDomain
	Organization           *snapshotOrganization
}

// formStatus is a form a portal uses.
type formStatus struct {
	ID     string
	Exists bool
}

// snapshotOrganization is the organization the portal opens in.
type snapshotOrganization struct {
	ID   string
	Name string
	// Connections are the IDs of the connections enabled for the organization.
	Connections []string
}

// host returns where the portal is served from: the custom domain when it is ready.
func (s snapshot) host() string {
	if s.CustomDomain != nil && s.CustomDomain.IsReady() {
		return s.CustomDomain.Domain
	}

	return s.TenantDomain
}

func (p Portal) pages() []Page {
	if p.Pages == nil {
		return nil
	}

	return p.Pages.Standard
}

func (p Portal) homePage() string {
	if p.Pages == nil {
		return ""
	}

	return p.Pages.Default
}

func (p Portal) sidebar() []SidebarComponent {
	if p.Navigation == nil || p.Navigation.Sidebar == nil {
		return nil
	}

	return p.Navigation.Sidebar.Components
}

// children returns the components held by a section.
func (c Component) children() []Component {
	items, _ := c.Config["children"].([]any)
	children := make([]Component, 0, len(items))

	for _, item := range items {
		entry, isObject := item.(map[string]any)
		if !isObject {
			continue
		}

		child := Component{}
		child.Type, _ = entry["type"].(string)
		child.Config, _ = entry["config"].(map[string]any)
		children = append(children, child)
	}

	return children
}

// allComponents returns the components of the pages and the ones they hold, in the order they come.
func allComponents(pages []Page) []Component {
	var all []Component

	var visit func(components []Component)

	visit = func(components []Component) {
		for _, component := range components {
			all = append(all, component)
			visit(component.children())
		}
	}

	for _, page := range pages {
		visit(page.Components)
	}

	return all
}

// formIDsOf returns the IDs of the forms the pages use, each once.
func formIDsOf(portal Portal) []string {
	var ids []string

	for _, component := range allComponents(portal.pages()) {
		if component.Type != formComponentType {
			continue
		}

		if id, _ := component.Config["form_id"].(string); id != "" && !containsString(ids, id) {
			ids = append(ids, id)
		}
	}

	return ids
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}

	return false
}

const formComponentType = "page:component:auth0:form"

// readSnapshot reads the portal, and then what the audit needs to know about it. The reads do not
// depend on each other, so they are sent together.
func readSnapshot(ctx context.Context, api API, portalID, tenantDomain, organizationID string) (snapshot, error) {
	portal, err := api.ReadPortal(ctx, portalID)
	if err != nil {
		return snapshot{}, fmt.Errorf("read portal %q: %w", portalID, err)
	}

	result := snapshot{TenantDomain: tenantDomain, Portal: portal}

	reads := []func() error{
		func() (err error) { return result.readApplication(ctx, api) },
		func() (err error) { return result.readResourceServers(ctx, api) },
		func() (err error) { return result.readForms(ctx, api) },
		func() (err error) { return result.readCustomDomain(ctx, api) },
	}

	if organizationID != "" {
		reads = append(reads, func() error { return result.readOrganization(ctx, api, organizationID) })
	}

	// Each read fills a part of the result of its own.
	failures := make([]error, len(reads))
	forEachParallel(len(reads), func(index int) { failures[index] = reads[index]() })

	if err := errors.Join(failures...); err != nil {
		return snapshot{}, err
	}

	return result, nil
}

// readApplication reads the application, and when it exists its grants and connections. A portal can
// still point at an application that was deleted. That is a finding (P009) and not a failure of the
// audit, so a 404 means no application.
func (s *snapshot) readApplication(ctx context.Context, api API) error {
	if s.Portal.Client == nil || s.Portal.Client.ClientID == "" {
		return nil
	}

	clientID := s.Portal.Client.ClientID

	client, err := api.ReadClient(ctx, clientID)
	if IsStatus(err, http.StatusNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read application %q: %w", clientID, err)
	}

	var (
		grants      []ClientGrant
		connections []Connection
	)

	failures := make([]error, 2)
	forEachParallel(2, func(index int) {
		if index == 0 {
			grants, failures[0] = api.ListClientGrants(ctx, clientID)
		} else {
			connections, failures[1] = api.ListClientConnections(ctx, clientID)
		}
	})

	if err := errors.Join(failures...); err != nil {
		return fmt.Errorf("read the grants and connections of application %q: %w", clientID, err)
	}

	s.Application = &client
	s.Grants = grants
	s.ApplicationConnections = connections

	return nil
}

func (s *snapshot) readResourceServers(ctx context.Context, api API) error {
	servers, err := api.ListResourceServers(ctx)
	if err != nil {
		return fmt.Errorf("list the resource servers: %w", err)
	}

	for _, server := range servers {
		s.ResourceServers = append(s.ResourceServers, server.Identifier)
	}

	return nil
}

// readForms checks that each form of the portal exists. The API answers 400 for an ID it cannot match
// to a form, because the ID fails its schema, which is the same finding as a deleted form (P007), so
// both mean the form is missing.
func (s *snapshot) readForms(ctx context.Context, api API) error {
	ids := formIDsOf(s.Portal)
	statuses := make([]formStatus, len(ids))
	failures := make([]error, len(ids))

	forEachParallel(len(ids), func(index int) {
		_, err := api.ReadForm(ctx, ids[index])
		statuses[index] = formStatus{ID: ids[index], Exists: err == nil}

		if err != nil && !IsStatus(err, http.StatusNotFound, http.StatusBadRequest) {
			failures[index] = fmt.Errorf("read form %q: %w", ids[index], err)
		}
	})

	if err := errors.Join(failures...); err != nil {
		return err
	}

	s.Forms = statuses

	return nil
}

func (s *snapshot) readCustomDomain(ctx context.Context, api API) error {
	domain, err := api.ReadDefaultCustomDomain(ctx)
	if IsStatus(err, http.StatusNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read the custom domain: %w", err)
	}

	s.CustomDomain = &domain

	return nil
}

func (s *snapshot) readOrganization(ctx context.Context, api API, organizationID string) error {
	organization, err := api.ReadOrganization(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("read organization %q: %w", organizationID, err)
	}

	connections, err := api.ListOrganizationConnections(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("list the connections of organization %q: %w", organizationID, err)
	}

	s.Organization = &snapshotOrganization{ID: organization.ID, Name: organization.Name, Connections: connections}

	return nil
}

// componentLabel is the name of a component in what the audit shows.
func componentLabel(component Component) string {
	name := component.Type
	if index := strings.LastIndex(name, ":"); index >= 0 {
		name = name[index+1:]
	}

	return strings.ReplaceAll(name, "_", " ")
}

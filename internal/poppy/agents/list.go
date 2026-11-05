package agents

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// Agent is a registered agent, with the status of its app and its grants.
type Agent struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	// RecordURL is the external_agent_id of the agent record, empty without a record.
	RecordURL      string         `json:"-"`
	ClientID       string         `json:"client_id"`
	MetadataStatus string         `json:"metadata_status"`
	DPoPRequired   Maybe[bool]    `json:"dpop_required"`
	APIs           Maybe[[]Grant] `json:"apis"`
}

// isPoppy reports whether the agent has a grant to a Poppy API. An agent whose
// grants could not be read counts, because it may have one.
func (a Agent) isPoppy() bool {
	return !a.APIs.Known || len(a.APIs.Value) > 0
}

// entry is an agent record and the agent app linked to it. Either can be missing.
type entry struct {
	record *managementv3.AgentResponseContent
	client *managementv3.Client
}

func (e entry) isAgentApp() bool {
	return e.client != nil && e.client.GetExternalMetadataType() == managementv3.ClientExternalMetadataTypeEnumCimd
}

func (s *Service) listRecords(ctx context.Context) ([]*managementv3.AgentResponseContent, error) {
	page, err := s.Agents.List(ctx, &managementv3.ListAgentsRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	records, err := collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return records, nil
}

// listAgentApps returns every client that is an agent app.
func (s *Service) listAgentApps(ctx context.Context) ([]*managementv3.Client, error) {
	include := true
	fields := clientFields

	page, err := s.Clients.List(ctx, &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include})
	if err != nil {
		return nil, err
	}

	clients, err := collect(ctx, page)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(clients, func(client *managementv3.Client) bool {
		return !entry{client: client}.isAgentApp()
	}), nil
}

// poppyAudiences returns the identifiers of the Poppy APIs.
func (s *Service) poppyAudiences(ctx context.Context) (map[string]bool, error) {
	apis, err := s.poppyAPIs(ctx)
	if err != nil {
		return nil, err
	}

	audiences := make(map[string]bool, len(apis))
	for _, api := range apis {
		audiences[api.Identifier] = true
	}

	return audiences, nil
}

// describe reads the grants of an entry's agent app. A value it cannot read
// stays unknown. When clientsKnown is true, an entry without an agent app is
// known to have no grants.
func (s *Service) describe(ctx context.Context, e entry, clientsKnown bool, poppy map[string]bool) Agent {
	agent := Agent{MetadataStatus: unknown}

	if e.record != nil {
		agent.AgentID, agent.Name, agent.URL, agent.RecordURL = e.record.GetAgentID(), e.record.GetName(), e.record.GetExternalAgentID(), e.record.GetExternalAgentID()
	}

	if e.client == nil {
		if clientsKnown {
			agent.APIs = known([]Grant{})
		}

		return agent
	}

	agent.ClientID = e.client.GetClientID()
	agent.Name = cmp.Or(agent.Name, e.client.GetName())
	agent.URL = cmp.Or(e.client.GetExternalClientID(), agent.URL)
	agent.MetadataStatus = cmp.Or(string(e.client.GetExternalMetadataStatus()), unknown)
	agent.DPoPRequired = known(e.client.GetRequireProofOfPossession())

	grants, err := s.grantsOf(ctx, agent.ClientID)
	if err != nil {
		return agent
	}

	apis := []Grant{}
	for _, grant := range grants {
		if poppy[grant.Audience] {
			apis = append(apis, grant)
		}
	}
	agent.APIs = known(apis)

	return agent
}

// List returns the Poppy agents: the agents whose app has a grant to a Poppy
// API. With all set it also returns the agents that have none. A value that
// cannot be read, such as the grants of one agent, is reported as unknown.
func (s *Service) List(ctx context.Context, all bool) ([]Agent, error) {
	records, err := s.listRecords(ctx)
	if err != nil {
		return nil, err
	}

	poppy, err := s.poppyAudiences(ctx)
	if err != nil {
		return nil, err
	}

	apps, appsErr := s.listAgentApps(ctx)

	var entries []entry
	linked := make(map[string]bool, len(records))

	for _, record := range records {
		linked[record.GetAgentID()] = true

		i := slices.IndexFunc(apps, func(client *managementv3.Client) bool { return client.GetAgentID() == record.GetAgentID() })
		e := entry{record: record}
		if i >= 0 {
			e.client = apps[i]
		}
		entries = append(entries, e)
	}

	for _, app := range apps {
		if !linked[app.GetAgentID()] {
			entries = append(entries, entry{client: app})
		}
	}

	agents := []Agent{}
	for _, e := range entries {
		agent := s.describe(ctx, e, appsErr == nil, poppy)
		if all || agent.isPoppy() {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// find returns the agent app and agent record for an agent URL or agent ID. It
// returns nil when no client matches.
func (s *Service) find(ctx context.Context, ref string) (*entry, error) {
	include := true
	fields := clientFields
	request := &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include}

	if isURL(ref) {
		request.ExternalClientID = &ref
	} else {
		request.AgentID = &ref
	}

	page, err := s.Clients.List(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", ref, err)
	}

	clients, err := collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", ref, err)
	}

	// The server-side filter is not trusted: a lookup that leads to a write or a
	// removal uses only the client that is the one asked for.
	clients = slices.DeleteFunc(clients, func(c *managementv3.Client) bool {
		if isURL(ref) {
			return c.GetExternalClientID() != ref
		}

		return c.GetAgentID() != ref
	})

	if len(clients) == 0 {
		return nil, nil
	}

	e := &entry{client: clients[0]}

	if agentID := e.client.GetAgentID(); agentID != "" {
		e.record, err = s.Agents.Read(ctx, agentID)
		if statusOf(err) == 404 {
			e.record, err = nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read agent %s: %w", agentID, err)
		}
	}

	return e, nil
}

// MetadataCheck is the live check of an agent's metadata document.
type MetadataCheck struct {
	Valid      Maybe[bool] `json:"valid"`
	Violations []string    `json:"violations"`
	Warnings   []string    `json:"warnings"`
}

// Detail is an agent with a live check of its metadata document.
type Detail struct {
	Agent
	MetadataCheck MetadataCheck `json:"metadata_check"`
	// KeysCurrent tells whether the keys Auth0 stored match the document's keys.
	KeysCurrent Maybe[bool] `json:"keys_current"`
	// Notes explain a check that could not be made.
	Notes []string `json:"notes"`
}

// Show returns one Poppy agent, given its URL or agent ID, with a live check of
// its metadata document and of whether its stored keys are current. A check
// that fails is reported in Notes, not as an error.
func (s *Service) Show(ctx context.Context, ref string) (*Detail, error) {
	e, err := s.find(ctx, ref)
	if err != nil {
		return nil, err
	}

	if e == nil {
		return nil, fmt.Errorf("no agent found for %s", ref)
	}

	poppy, err := s.poppyAudiences(ctx)
	if err != nil {
		return nil, err
	}

	detail := &Detail{
		Agent:         s.describe(ctx, *e, true, poppy),
		MetadataCheck: MetadataCheck{Violations: []string{}, Warnings: []string{}},
		Notes:         []string{},
	}

	if !e.isAgentApp() || !detail.isPoppy() {
		return nil, fmt.Errorf("%s isn't a Poppy agent", ref)
	}

	s.checkMetadata(ctx, detail)

	return detail, nil
}

// checkMetadata previews the agent's document and compares its keys with the
// keys Auth0 stored.
func (s *Service) checkMetadata(ctx context.Context, detail *Detail) {
	preview, err := s.Preview(ctx, detail.URL)

	var invalid *InvalidDocumentError
	if errors.As(err, &invalid) {
		detail.MetadataCheck.Valid = known(false)
		detail.MetadataCheck.Violations = invalid.Violations

		return
	}

	if err != nil {
		detail.Notes = append(detail.Notes, fmt.Sprintf("The metadata document could not be checked: %v", err))

		return
	}

	detail.MetadataCheck.Valid = known(true)
	detail.MetadataCheck.Warnings = preview.Warnings

	credentials, err := s.Credentials.List(ctx, detail.ClientID)
	if err != nil {
		detail.Notes = append(detail.Notes, fmt.Sprintf("The keys Auth0 stored could not be read, so they were not compared: %v", err))

		return
	}

	stored := make([]string, len(credentials))
	for i, credential := range credentials {
		stored[i] = credential.GetKid()
	}

	current := make([]string, len(preview.Keys))
	for i, key := range preview.Keys {
		current[i] = key.KID
	}

	slices.Sort(stored)
	slices.Sort(current)
	detail.KeysCurrent = known(slices.Equal(slices.Compact(stored), slices.Compact(current)))
}

package agents

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// Agent is a registered agent, with the status of its app.
type Agent struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	// RecordURL is the external_agent_id of the agent record, empty without a record.
	RecordURL      string      `json:"-"`
	ClientID       string      `json:"client_id"`
	MetadataStatus string      `json:"metadata_status"`
	DPoPRequired   Maybe[bool] `json:"dpop_required"`
}

// Entry is an agent record and the agent app linked to it. Either can be missing.
type Entry struct {
	Record *managementv3.AgentResponseContent
	Client *managementv3.Client
}

// IsAgentApp reports whether the entry has an agent app.
func (e Entry) IsAgentApp() bool {
	return e.Client != nil && e.Client.GetExternalMetadataType() == managementv3.ClientExternalMetadataTypeEnumCimd
}

// Records returns every agent record.
func (s *Service) Records(ctx context.Context) ([]*managementv3.AgentResponseContent, error) {
	page, err := s.Agents.List(ctx, &managementv3.ListAgentsRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	records, err := Collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return records, nil
}

// RecordByURL returns the agent record whose external agent ID is the URL, nil
// when there is none. With several, it returns the first the agents API lists.
func (s *Service) RecordByURL(ctx context.Context, url string) (*managementv3.AgentResponseContent, error) {
	records, err := s.Records(ctx)
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(records, func(record *managementv3.AgentResponseContent) bool { return record.GetExternalAgentID() == url })
	if i < 0 {
		return nil, nil
	}

	return records[i], nil
}

// listClients returns every client, limited to the fields the commands read.
func (s *Service) listClients(ctx context.Context) ([]*managementv3.Client, error) {
	include := true
	fields := clientFields

	page, err := s.Clients.List(ctx, &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include})
	if err != nil {
		return nil, err
	}

	return Collect(ctx, page)
}

// listAgentApps returns every client that is an agent app.
func (s *Service) listAgentApps(ctx context.Context) ([]*managementv3.Client, error) {
	clients, err := s.listClients(ctx)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(clients, func(client *managementv3.Client) bool {
		return !Entry{Client: client}.IsAgentApp()
	}), nil
}

// LinkApps pairs each of the records with the client whose agent ID is the
// record's, whatever the type of that client. The entries are in the order of
// the records. The error says why the clients could not be listed, in which case
// no entry has a client.
func (s *Service) LinkApps(ctx context.Context, records []*managementv3.AgentResponseContent) ([]Entry, error) {
	clients, err := s.listClients(ctx)

	entries := make([]Entry, 0, len(records))

	for _, record := range records {
		e := Entry{Record: record}

		if i := slices.IndexFunc(clients, func(client *managementv3.Client) bool { return client.GetAgentID() == record.GetAgentID() }); i >= 0 {
			e.Client = clients[i]
		}

		entries = append(entries, e)
	}

	return entries, err
}

// Link pairs each of the records with its agent app, and adds the agent apps
// that have no record. The apps are known when they could be listed; when they
// could not, only the records are returned and known is false.
func (s *Service) Link(ctx context.Context, records []*managementv3.AgentResponseContent) (entries []Entry, known bool) {
	apps, appsErr := s.listAgentApps(ctx)

	linked := make(map[string]bool, len(records))

	for _, record := range records {
		linked[record.GetAgentID()] = true

		i := slices.IndexFunc(apps, func(client *managementv3.Client) bool { return client.GetAgentID() == record.GetAgentID() })
		e := Entry{Record: record}
		if i >= 0 {
			e.Client = apps[i]
		}
		entries = append(entries, e)
	}

	for _, app := range apps {
		if !linked[app.GetAgentID()] {
			entries = append(entries, Entry{Client: app})
		}
	}

	return entries, appsErr == nil
}

// Describe reads an entry's record and agent app. A value it cannot read stays
// unknown.
func Describe(e Entry) Agent {
	agent := Agent{MetadataStatus: unknown}

	if e.Record != nil {
		agent.AgentID, agent.Name, agent.URL, agent.RecordURL = e.Record.GetAgentID(), e.Record.GetName(), e.Record.GetExternalAgentID(), e.Record.GetExternalAgentID()
	}

	if e.Client == nil {
		return agent
	}

	agent.ClientID = e.Client.GetClientID()
	agent.Name = cmp.Or(agent.Name, e.Client.GetName())
	agent.URL = cmp.Or(e.Client.GetExternalClientID(), agent.URL)
	agent.MetadataStatus = cmp.Or(string(e.Client.GetExternalMetadataStatus()), unknown)
	agent.DPoPRequired = Known(e.Client.GetRequireProofOfPossession())

	return agent
}

// FindApp returns the agent app for an agent URL or agent ID. It returns nil
// when no client matches.
func (s *Service) FindApp(ctx context.Context, ref string) (*managementv3.Client, error) {
	include := true
	fields := clientFields
	request := &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include}

	if IsURL(ref) {
		request.ExternalClientID = &ref
	} else {
		request.AgentID = &ref
	}

	page, err := s.Clients.List(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", ref, err)
	}

	clients, err := Collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", ref, err)
	}

	// The server-side filter is not trusted: a lookup that leads to a write or a
	// removal uses only the client that is the one asked for.
	clients = slices.DeleteFunc(clients, func(c *managementv3.Client) bool {
		if IsURL(ref) {
			return c.GetExternalClientID() != ref
		}

		return c.GetAgentID() != ref
	})

	if len(clients) == 0 {
		return nil, nil
	}

	return clients[0], nil
}

// Find returns the agent app and agent record for an agent URL or agent ID. It
// returns nil when no client matches.
func (s *Service) Find(ctx context.Context, ref string) (*Entry, error) {
	client, err := s.FindApp(ctx, ref)
	if err != nil || client == nil {
		return nil, err
	}

	e := &Entry{Client: client}

	if agentID := e.Client.GetAgentID(); agentID != "" {
		e.Record, err = s.Agents.Read(ctx, agentID)
		if statusOf(err) == 404 {
			e.Record, err = nil, nil
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
	// UsableKeys is how many keys of the document Auth0 stores, as the preview
	// reports them.
	UsableKeys int `json:"-"`
}

// Detail is an agent with a live check of its metadata document.
type Detail struct {
	Agent
	MetadataCheck MetadataCheck `json:"metadata_check"`
	// KeysCurrent tells whether the keys Auth0 stored match the document's keys.
	KeysCurrent Maybe[bool] `json:"keys_current"`
	// Notes explain a check that could not be made.
	Notes []string `json:"notes"`
	// ReadErrors are why a check could not be made, for the caller to report.
	ReadErrors []error `json:"-"`
}

// NewDetail returns the detail of an agent, before any check is made.
func NewDetail(agent Agent) *Detail {
	return &Detail{
		Agent:         agent,
		MetadataCheck: MetadataCheck{Violations: []string{}, Warnings: []string{}},
		Notes:         []string{},
	}
}

// CheckMetadata previews the agent's document and compares its keys with the
// keys Auth0 stored. A check that fails is reported in Notes, not as an error.
func (s *Service) CheckMetadata(ctx context.Context, detail *Detail) {
	preview, err := s.Preview(ctx, detail.URL)

	var invalid *InvalidDocumentError
	if errors.As(err, &invalid) {
		detail.MetadataCheck.Valid = Known(false)
		detail.MetadataCheck.Violations = invalid.Violations

		return
	}

	if err != nil {
		detail.Notes = append(detail.Notes, fmt.Sprintf("The metadata document could not be checked: %v", err))
		detail.ReadErrors = append(detail.ReadErrors, err)

		return
	}

	detail.MetadataCheck.Valid = Known(true)
	detail.MetadataCheck.Warnings = preview.Warnings

	credentials, err := s.Credentials.List(ctx, detail.ClientID)
	if err != nil {
		detail.Notes = append(detail.Notes, fmt.Sprintf("The keys Auth0 stored could not be read, so they were not compared: %v", err))
		detail.ReadErrors = append(detail.ReadErrors, err)

		return
	}

	stored := make([]string, len(credentials))
	for i, credential := range credentials {
		stored[i] = credential.GetKid()
	}

	// The preview lists only the keys Auth0 stores, so all of them are compared
	// with the stored ones.
	current := make([]string, len(preview.Keys))
	for i, key := range preview.Keys {
		current[i] = key.KID
	}

	detail.MetadataCheck.UsableKeys = len(preview.Keys)

	slices.Sort(stored)
	slices.Sort(current)
	detail.KeysCurrent = Known(slices.Equal(slices.Compact(stored), slices.Compact(current)))
}

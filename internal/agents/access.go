package agents

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// errUnwired is why the extra data is unknown when the Management API clients are not set.
var errUnwired = errors.New("the Management API clients are not set")

// noApp is the metadata status of an agent record that has no agent app.
const noApp = "none"

// notApplicable is the metadata status of a linked app that is not an agent
// app, which has no metadata document.
const notApplicable = "-"

// AudienceFilter returns the audiences of the APIs a read is narrowed to. It is
// called once per read. A nil filter covers every API.
type AudienceFilter func(ctx context.Context) (map[string]bool, error)

// audiences is an AudienceFilter that was called.
type audiences struct {
	only     map[string]bool
	narrowed bool
	// Err is why the filter could not be applied.
	err error
}

func resolve(ctx context.Context, only AudienceFilter) audiences {
	if only == nil {
		return audiences{}
	}

	set, err := only(ctx)

	return audiences{only: set, narrowed: true, err: err}
}

// has tells whether the API with the audience is covered.
func (a audiences) has(audience string) bool {
	return !a.narrowed || a.only[audience]
}

// GrantedAgent is an agent, with the status of its app and its grants.
type GrantedAgent struct {
	Agent
	APIs Maybe[[]Grant] `json:"apis"`
}

// MayHaveGrants reports whether the agent has a grant. An agent whose grants
// could not be read counts, because it may have one.
func (a GrantedAgent) MayHaveGrants() bool {
	return !a.APIs.Known || len(a.APIs.Value) > 0
}

// describeGranted reads the grants of an entry's agent app. A value it cannot
// read stays unknown, and the error says why. When clientsKnown is true, an
// entry without an agent app is known to have no grants. Grants on APIs the
// audiences do not cover are left out; when the audiences could not be read,
// there is no telling which grants count, so they are unknown.
func (s *Service) describeGranted(ctx context.Context, e Entry, clientsKnown bool, a audiences) (GrantedAgent, error) {
	agent := GrantedAgent{Agent: Describe(e)}

	if e.Client == nil || a.err != nil {
		return attachGrants(agent, e, clientsKnown, a, nil, a.err), a.err
	}

	grants, err := s.GrantsOf(ctx, agent.ClientID)

	return attachGrants(agent, e, clientsKnown, a, grants, err), err
}

// attachGrants sets the APIs of an agent from the grants of its app, which are
// unknown when grantsErr says they could not be read.
func attachGrants(agent GrantedAgent, e Entry, clientsKnown bool, a audiences, grants []Grant, grantsErr error) GrantedAgent {
	if e.Client == nil {
		if clientsKnown {
			agent.APIs = Known([]Grant{})
		}

		return agent
	}

	if a.err != nil || grantsErr != nil {
		return agent
	}

	apis := []Grant{}
	for _, grant := range grants {
		if a.has(grant.Audience) {
			apis = append(apis, grant)
		}
	}
	agent.APIs = Known(apis)

	return agent
}

// ListGranted returns every agent, with the grants of its app on the APIs the
// filter covers (every API when it is nil). A value that cannot be read, such as
// the grants of one agent, is reported as unknown; the filter failing is an error.
func (s *Service) ListGranted(ctx context.Context, only AudienceFilter) ([]GrantedAgent, error) {
	records, err := s.Records(ctx)
	if err != nil {
		return nil, err
	}

	a := resolve(ctx, only)
	if a.err != nil {
		return nil, a.err
	}

	entries, appsKnown := s.Link(ctx, records)

	agents := make([]GrantedAgent, len(entries))
	for i, e := range entries {
		agents[i], _ = s.describeGranted(ctx, e, appsKnown, a)
	}

	return agents, nil
}

// FindGranted returns the agent app and agent record for an agent URL or agent
// ID, with the grants of the app on the APIs the filter covers. It fails when no
// agent is found.
func (s *Service) FindGranted(ctx context.Context, ref string, only AudienceFilter) (*Entry, GrantedAgent, error) {
	e, err := s.Find(ctx, ref)
	if err != nil {
		return nil, GrantedAgent{}, err
	}

	if e == nil {
		return nil, GrantedAgent{}, fmt.Errorf("no agent found for %s", ref)
	}

	a := resolve(ctx, only)
	if a.err != nil {
		return nil, GrantedAgent{}, a.err
	}

	agent, _ := s.describeGranted(ctx, *e, true, a)

	return e, agent, nil
}

// GrantedDetail is an agent with its grants and a live check of its metadata
// document.
type GrantedDetail struct {
	GrantedAgent
	MetadataCheck MetadataCheck `json:"metadata_check"`
	// KeysCurrent tells whether the keys Auth0 stored match the document's keys.
	KeysCurrent Maybe[bool] `json:"keys_current"`
	// Notes explain a check that could not be made.
	Notes []string `json:"notes"`
}

// CheckGranted checks the metadata document of an agent and whether its stored
// keys are current. A check that fails is reported in Notes, not as an error.
func (s *Service) CheckGranted(ctx context.Context, agent GrantedAgent) *GrantedDetail {
	checked := NewDetail(agent.Agent)
	s.CheckMetadata(ctx, checked)

	return &GrantedDetail{
		GrantedAgent:  agent,
		MetadataCheck: checked.MetadataCheck,
		KeysCurrent:   checked.KeysCurrent,
		Notes:         checked.Notes,
	}
}

// Extras is what `agents list` and `agents show` add to an agent record of the
// agents API: the app linked to it and the APIs it has user grants on. A value
// that could not be read is unknown (null in the JSON) and named in
// UnknownFields, so a missing scope never fails the command.
type Extras struct {
	// ClientID is empty when the agent has no linked app.
	ClientID       Maybe[string]  `json:"client_id"`
	MetadataStatus Maybe[string]  `json:"metadata_status"`
	DPoPRequired   Maybe[bool]    `json:"dpop_required"`
	APIs           Maybe[[]Grant] `json:"apis"`
	// UnknownFields lists the fields that could not be read.
	UnknownFields []string `json:"unknown_fields"`
	// NotAgentApp tells that the linked app is not an agent app, so it has no
	// metadata document to check.
	NotAgentApp bool `json:"-"`
}

// HasApp tells whether the agent is known to have a linked app.
func (x Extras) HasApp() bool { return x.ClientID.Known && x.ClientID.Value != "" }

// unknownFields names the fields that could not be read. The DPoP setting of an
// agent that has no app does not apply, so it is not one of them.
func (x Extras) unknownFields() []string {
	names := []string{}

	if !x.ClientID.Known {
		names = append(names, "client_id")
	}

	if !x.MetadataStatus.Known {
		names = append(names, "metadata_status")
	}

	if !x.DPoPRequired.Known && (!x.ClientID.Known || x.ClientID.Value != "") {
		names = append(names, "dpop_required")
	}

	if !x.APIs.Known {
		names = append(names, "apis")
	}

	return names
}

// DetailExtras is Extras with a live check of the agent's metadata document.
type DetailExtras struct {
	Extras
	MetadataCheck MetadataCheck `json:"metadata_check"`
	// KeysCurrent tells whether the keys Auth0 stored match the document's keys.
	KeysCurrent Maybe[bool] `json:"keys_current"`
	// Notes explain a check that could not be made.
	Notes []string `json:"notes"`
	// ReadErrors are why values could not be read, for the caller to report.
	ReadErrors []error `json:"-"`
}

// markUnknown sets UnknownFields from the values that are unknown. The metadata
// check applies to an agent app only.
func (d *DetailExtras) markUnknown() {
	d.UnknownFields = d.unknownFields()

	if d.ClientID.Known && (!d.HasApp() || d.NotAgentApp) {
		return
	}

	if !d.MetadataCheck.Valid.Known {
		d.UnknownFields = append(d.UnknownFields, "metadata_check.valid")
	}

	if !d.KeysCurrent.Known {
		d.UnknownFields = append(d.UnknownFields, "keys_current")
	}
}

// wired tells whether every API the extra data needs is there.
func (s *Service) wired() bool {
	return s != nil && s.Clients != nil && s.Credentials != nil && s.Grants != nil
}

// onlyUserGrants keeps the grants that let the agent act for a person.
func onlyUserGrants(apis Maybe[[]Grant]) Maybe[[]Grant] {
	if !apis.Known {
		return apis
	}

	users := []Grant{}
	for _, grant := range apis.Value {
		if grant.IsUserGrant() {
			users = append(users, grant)
		}
	}

	return Known(users)
}

// extrasOf turns a described entry into the extra data. AppsKnown tells whether
// the apps could be listed. Only the grants for users count as the APIs of the
// agent: a grant of another subject type does not let it act for a person.
func extrasOf(agent GrantedAgent, e Entry, appsKnown bool) Extras {
	var x Extras

	switch {
	case e.Client != nil && !e.IsAgentApp():
		x = Extras{
			ClientID:       Known(agent.ClientID),
			MetadataStatus: Known(notApplicable),
			DPoPRequired:   agent.DPoPRequired,
			APIs:           onlyUserGrants(agent.APIs),
			NotAgentApp:    true,
		}
	case e.Client != nil:
		x = Extras{
			ClientID:       Known(agent.ClientID),
			MetadataStatus: Known(agent.MetadataStatus),
			DPoPRequired:   agent.DPoPRequired,
			APIs:           onlyUserGrants(agent.APIs),
		}
	case appsKnown:
		x = Extras{ClientID: Known(""), MetadataStatus: Known(noApp), APIs: onlyUserGrants(agent.APIs)}
	}

	x.UnknownFields = x.unknownFields()

	return x
}

// Extras reads the extra data of agent records, in the same order. It lists the
// clients once and the grants of the tenant once, however many agents there are;
// the filter, when there is one, is called once. Only the grants on the APIs it
// covers are reported, every grant when it is nil. Whatever cannot be read is
// unknown, and the error says why: it never stops the other values from being
// read.
func (s *Service) Extras(ctx context.Context, records []*managementv3.AgentResponseContent, only AudienceFilter) ([]Extras, error) {
	extras := make([]Extras, len(records))

	if len(records) == 0 {
		return extras, nil
	}

	if !s.wired() {
		for i := range extras {
			extras[i] = extrasOf(GrantedAgent{}, Entry{}, false)
		}

		return extras, errUnwired
	}

	a := resolve(ctx, only)

	// The app of a record is the client with its agent ID, of any type.
	entries, linkErr := s.LinkApps(ctx, records)
	appsKnown := linkErr == nil

	var grants map[string][]Grant
	var grantsErr error

	if slices.ContainsFunc(entries, func(e Entry) bool { return e.Client != nil }) && a.err == nil {
		grants, grantsErr = s.GrantsByClient(ctx)
	}

	for i, e := range entries {
		agent := GrantedAgent{Agent: Describe(e)}
		agent = attachGrants(agent, e, appsKnown, a, grants[agent.ClientID], grantsErr)
		extras[i] = extrasOf(agent, e, appsKnown)
	}

	return extras, errors.Join(linkErr, a.err, grantsErr)
}

// LookupApp looks up the agent app linked to an agent ID, nil when there is
// none. It fails when the lookup cannot be made.
func (s *Service) LookupApp(ctx context.Context, agentID string) (*managementv3.Client, error) {
	if !s.wired() {
		return nil, fmt.Errorf("the agent app of %s could not be looked up", agentID)
	}

	return s.FindApp(ctx, agentID)
}

// Inspect reads the extra data of one agent record, with a live check of its
// metadata document and of whether its stored keys are current. Client is the
// app linked to the record, nil when it has none; lookupErr is why the app could
// not be looked up, nil when it could. A check that fails is reported in Notes.
// Only narrows the grants like Extras does.
func (s *Service) Inspect(ctx context.Context, record *managementv3.AgentResponseContent, client *managementv3.Client, lookupErr error, only AudienceFilter) *DetailExtras {
	detail := &DetailExtras{
		MetadataCheck: MetadataCheck{Violations: []string{}, Warnings: []string{}},
		Notes:         []string{},
	}

	if lookupErr != nil || !s.wired() {
		detail.Notes = append(detail.Notes, fmt.Sprintf("The agent app could not be read, so the metadata document was not checked: %v", cmp.Or(lookupErr, errUnwired)))
		detail.ReadErrors = []error{cmp.Or(lookupErr, errUnwired)}
		detail.markUnknown()

		return detail
	}

	entry := Entry{Record: record, Client: client}

	agent, grantsErr := s.describeGranted(ctx, entry, true, resolve(ctx, only))
	detail.Extras = extrasOf(agent, entry, true)

	if grantsErr != nil {
		detail.ReadErrors = append(detail.ReadErrors, grantsErr)
	}

	// An app that is not an agent app has no metadata document to check.
	if entry.Client == nil || !entry.IsAgentApp() {
		detail.markUnknown()

		return detail
	}

	checked := NewDetail(agent.Agent)
	s.CheckMetadata(ctx, checked)

	detail.MetadataCheck, detail.KeysCurrent, detail.Notes = checked.MetadataCheck, checked.KeysCurrent, checked.Notes
	detail.ReadErrors = append(detail.ReadErrors, checked.ReadErrors...)

	// A list that is empty is [] in the JSON, never null.
	if detail.MetadataCheck.Violations == nil {
		detail.MetadataCheck.Violations = []string{}
	}
	if detail.MetadataCheck.Warnings == nil {
		detail.MetadataCheck.Warnings = []string{}
	}

	detail.markUnknown()

	return detail
}

package agents

import (
	"context"
	"fmt"
	"slices"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// NotAnAgentError reports a reference that is not an agent to remove, so
// nothing was removed.
type NotAnAgentError struct {
	Ref string
	// AgentID is the agent record linked to the app, when the app is not an agent
	// app. It is empty when NoGrants is set.
	AgentID string
	// NoGrants is true when the app is an agent app but has no grant on the APIs
	// the removal was narrowed to, and has grants on other APIs.
	NoGrants bool
}

func (e *NotAnAgentError) Error() string {
	if e.NoGrants {
		return e.Ref + " has no grant to remove. Nothing removed."
	}

	return e.Ref + " isn't an agent. Nothing removed."
}

// Removal is what removing an agent would touch.
type Removal struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	// ClientID is empty when the agent record has no agent app.
	ClientID string `json:"client_id"`
	// AgentID is empty when the agent app has no agent record.
	AgentID string `json:"agent_id"`
	// Gone is true when neither an agent app nor an agent record exists for the
	// reference any more.
	Gone bool `json:"gone"`
	// Grants are the app's grants to remove. OtherGrants are the grants that a
	// removal narrowed to some APIs leaves alone; it is empty otherwise.
	Grants      []Grant `json:"grants"`
	OtherGrants []Grant `json:"other_grants"`
}

// PlanRemoval finds the agent app, its agent record and its grants for an agent
// URL or agent ID. Only narrows the removal to the grants on some APIs; nil
// covers every grant. It returns a *NotAnAgentError when the app is not an agent
// app, or when a narrowed removal finds no grant on those APIs but grants on
// others. A narrowed removal of an agent that has no grant at all plans the
// removal of its record and app, which is where a removal that stopped partway
// is picked up. An agent record without an app is planned on its own. The
// Removal has Gone set when there is neither.
func (s *Service) PlanRemoval(ctx context.Context, ref string, only AudienceFilter) (*Removal, error) {
	e, err := s.Find(ctx, ref)
	if err != nil {
		return nil, err
	}

	if e == nil {
		return s.planRecordRemoval(ctx, ref)
	}

	if !e.IsAgentApp() {
		return nil, &NotAnAgentError{Ref: ref, AgentID: e.Client.GetAgentID()}
	}

	audiences := resolve(ctx, only)
	if audiences.err != nil {
		return nil, audiences.err
	}

	clientID := e.Client.GetClientID()

	grants, err := s.GrantsOf(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to list the grants of %s: %w", clientID, err)
	}

	removal := &Removal{
		URL:         e.Client.GetExternalClientID(),
		Name:        e.Client.GetName(),
		ClientID:    clientID,
		Grants:      []Grant{},
		OtherGrants: []Grant{},
	}

	if e.Record != nil {
		removal.AgentID, removal.Name = e.Record.GetAgentID(), e.Record.GetName()
	}

	for _, grant := range grants {
		if audiences.has(grant.Audience) {
			removal.Grants = append(removal.Grants, grant)
		} else {
			removal.OtherGrants = append(removal.OtherGrants, grant)
		}
	}

	if audiences.narrowed && len(removal.Grants) == 0 && len(removal.OtherGrants) > 0 {
		return nil, &NotAnAgentError{Ref: ref, NoGrants: true}
	}

	return removal, nil
}

// planRecordRemoval plans the removal of an agent record that has no agent app.
// It returns a Removal with Gone set when there is no such record either.
func (s *Service) planRecordRemoval(ctx context.Context, ref string) (*Removal, error) {
	var (
		record *managementv3.AgentResponseContent
		err    error
	)

	if IsURL(ref) {
		record, err = s.RecordByURL(ctx, ref)
	} else {
		record, err = s.Agents.Read(ctx, ref)
		if statusOf(err) == 404 {
			record, err = nil, nil
		}

		if err != nil {
			err = fmt.Errorf("failed to read agent %s: %w", ref, err)
		}
	}

	if err != nil {
		return nil, err
	}

	if record == nil {
		removal := &Removal{Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}}
		if IsURL(ref) {
			removal.URL = ref
		}

		return removal, nil
	}

	return &Removal{
		URL:         record.GetExternalAgentID(),
		Name:        record.GetName(),
		AgentID:     record.GetAgentID(),
		Grants:      []Grant{},
		OtherGrants: []Grant{},
	}, nil
}

// Candidates returns the agents that can be unregistered: the agents that have
// an agent app. With only, it leaves out those that have grants but none on the
// APIs it covers, since a removal narrowed to them would find nothing to remove.
func (s *Service) Candidates(ctx context.Context, only AudienceFilter) ([]Entry, error) {
	apps, err := s.listAgentApps(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list the agent apps: %w", err)
	}

	audiences := resolve(ctx, only)
	if audiences.err != nil {
		return nil, audiences.err
	}

	records, err := s.Records(ctx)
	if err != nil {
		return nil, err
	}

	var grants map[string][]Grant

	if audiences.narrowed {
		if grants, err = s.GrantsByClient(ctx); err != nil {
			return nil, fmt.Errorf("failed to list the grants: %w", err)
		}
	}

	var entries []Entry

	for _, app := range apps {
		if audiences.narrowed {
			held := grants[app.GetClientID()]
			if len(held) > 0 && !slices.ContainsFunc(held, func(grant Grant) bool { return audiences.has(grant.Audience) }) {
				continue
			}
		}

		e := Entry{Client: app}
		if i := slices.IndexFunc(records, func(record *managementv3.AgentResponseContent) bool { return record.GetAgentID() == app.GetAgentID() }); i >= 0 && app.GetAgentID() != "" {
			e.Record = records[i]
		}

		entries = append(entries, e)
	}

	return entries, nil
}

// RemoveOptions chooses what Remove does.
type RemoveOptions struct {
	// DeleteApp also removes the agent record and the agent app. Without it only
	// the grants go.
	DeleteApp bool
}

// The kinds of thing a removal removes or keeps.
const (
	KindClientGrant = "client_grant"
	KindAgentRecord = "agent_record"
	KindAgentApp    = "agent_app"
)

// Item is a thing a removal removes or keeps. Audience is set for a client grant.
type Item struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Audience string `json:"audience,omitempty"`
}

// String describes the item for a person.
func (i Item) String() string {
	switch i.Kind {
	case KindClientGrant:
		return fmt.Sprintf("client grant %s (%s)", i.ID, i.Audience)
	case KindAgentRecord:
		return "agent record " + i.ID
	default:
		return "agent app " + i.ID
	}
}

// RemoveResult tells what was removed, what was kept and which grants remain.
type RemoveResult struct {
	URL             string  `json:"url"`
	DryRun          bool    `json:"dry_run"`
	Gone            bool    `json:"gone"`
	Removed         []Item  `json:"removed"`
	Kept            []Item  `json:"kept"`
	RemainingGrants []Grant `json:"remaining_grants"`
}

// removalSteps lists the changes of a removal in the order they are made: the
// grants, the agent record and the agent app. Items describes each step.
func (s *Service) removalSteps(removal *Removal, opts RemoveOptions) (steps []Step, items []Item) {
	for _, grant := range removal.Grants {
		item := Item{Kind: KindClientGrant, ID: grant.ID, Audience: grant.Audience}
		items = append(items, item)
		steps = append(steps, Step{Verb: "delete", Noun: item.String(), Run: func(ctx context.Context) error {
			return s.Grants.Delete(ctx, grant.ID)
		}})
	}

	if !opts.DeleteApp || removal.Gone {
		return steps, items
	}

	if removal.AgentID != "" {
		item := Item{Kind: KindAgentRecord, ID: removal.AgentID}
		items = append(items, item)
		steps = append(steps, Step{Verb: "delete", Noun: item.String(), Run: func(ctx context.Context) error {
			return s.Agents.Delete(ctx, removal.AgentID)
		}})
	}

	if removal.ClientID != "" {
		item := Item{Kind: KindAgentApp, ID: removal.ClientID}
		items = append(items, item)
		steps = append(steps, Step{Verb: "delete", Noun: item.String(), Run: func(ctx context.Context) error {
			return s.Clients.Delete(ctx, removal.ClientID)
		}})
	}

	return steps, items
}

// outcome describes a removal that goes through all of its steps.
func outcome(removal *Removal, opts RemoveOptions, items []Item) *RemoveResult {
	result := &RemoveResult{
		URL:             removal.URL,
		Gone:            removal.Gone,
		Removed:         append([]Item{}, items...),
		Kept:            []Item{},
		RemainingGrants: []Grant{},
	}

	if !opts.DeleteApp && !removal.Gone {
		result.RemainingGrants = removal.OtherGrants

		if removal.AgentID != "" {
			result.Kept = append(result.Kept, Item{Kind: KindAgentRecord, ID: removal.AgentID})
		}

		if removal.ClientID != "" {
			result.Kept = append(result.Kept, Item{Kind: KindAgentApp, ID: removal.ClientID})
		}
	}

	return result
}

// Plan reports what Remove would do, without changing anything.
func (s *Service) Plan(removal *Removal, opts RemoveOptions) *RemoveResult {
	_, items := s.removalSteps(removal, opts)

	result := outcome(removal, opts, items)
	result.DryRun = true

	return result
}

// Remove deletes the grants, then, with DeleteApp, the agent record and the
// agent app. It stops at the first failure; a failure after the first change
// returns a *PartialError.
func (s *Service) Remove(ctx context.Context, removal *Removal, opts RemoveOptions) (*RemoveResult, error) {
	steps, items := s.removalSteps(removal, opts)

	if err := RunSteps(ctx, steps); err != nil {
		return nil, err
	}

	return outcome(removal, opts, items), nil
}

// Approval says whether a removal may go ahead and what is needed first.
type Approval int

const (
	// Proceed means the removal can run now.
	Proceed Approval = iota
	// Ask means the user must confirm the plan.
	Ask
	// Refuse means the removal needs confirmation but cannot ask for it.
	Refuse
)

// Decision is the outcome of Decide.
type Decision struct {
	Approval Approval
	// DeleteApp is true when the removal also covers the agent record and the
	// agent app.
	DeleteApp bool
}

// Decide applies the removal rules. An agent that is already gone, or force,
// goes ahead. Otherwise a terminal is needed to confirm, and without one the
// removal is refused. The record and the app are removed only when no grant is
// left that the removal does not cover.
func Decide(removal *Removal, force, interactive bool) Decision {
	deleteApp := !removal.Gone && len(removal.OtherGrants) == 0

	switch {
	case removal.Gone || force:
		return Decision{Approval: Proceed, DeleteApp: deleteApp}
	case !interactive:
		return Decision{Approval: Refuse, DeleteApp: deleteApp}
	default:
		return Decision{Approval: Ask, DeleteApp: deleteApp}
	}
}

package agents

import (
	"context"
	"fmt"
	"slices"
)

// NotPoppyAgentError reports a reference that is not a Poppy agent, so nothing
// was removed.
type NotPoppyAgentError struct {
	Ref string
}

func (e *NotPoppyAgentError) Error() string {
	return e.Ref + " isn't a Poppy agent. Nothing removed."
}

// Removal is what removing an agent would touch.
type Removal struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	ClientID string `json:"client_id"`
	// AgentID is empty when the agent app has no agent record.
	AgentID string `json:"agent_id"`
	// Gone is true when no agent app exists for the reference any more.
	Gone bool `json:"gone"`
	// PoppyGrants are the app's grants to Poppy APIs, and OtherGrants the rest.
	PoppyGrants []Grant `json:"poppy_grants"`
	OtherGrants []Grant `json:"other_grants"`
}

// PlanRemoval finds the agent app, its agent record and its grants for an agent
// URL or agent ID. It returns a *NotPoppyAgentError when the app is not an agent
// app with a grant to a Poppy API, and a Removal with Gone set when there is no
// app at all.
func (s *Service) PlanRemoval(ctx context.Context, ref string) (*Removal, error) {
	e, err := s.find(ctx, ref)
	if err != nil {
		return nil, err
	}

	if e == nil {
		removal := &Removal{Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}}
		if isURL(ref) {
			removal.URL = ref
		}

		return removal, nil
	}

	if !e.isAgentApp() {
		return nil, &NotPoppyAgentError{Ref: ref}
	}

	poppy, err := s.poppyAudiences(ctx)
	if err != nil {
		return nil, err
	}

	clientID := e.client.GetClientID()

	grants, err := s.grantsOf(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to list the grants of %s: %w", clientID, err)
	}

	removal := &Removal{
		URL:         e.client.GetExternalClientID(),
		Name:        e.client.GetName(),
		ClientID:    clientID,
		PoppyGrants: []Grant{},
		OtherGrants: []Grant{},
	}

	if e.record != nil {
		removal.AgentID, removal.Name = e.record.GetAgentID(), e.record.GetName()
	}

	for _, grant := range grants {
		if poppy[grant.Audience] {
			removal.PoppyGrants = append(removal.PoppyGrants, grant)
		} else {
			removal.OtherGrants = append(removal.OtherGrants, grant)
		}
	}

	if len(removal.PoppyGrants) == 0 {
		return nil, &NotPoppyAgentError{Ref: ref}
	}

	return removal, nil
}

// RemoveOptions chooses what Remove does.
type RemoveOptions struct {
	// Everything also removes the app's other grants, the agent record and the
	// agent app. Without it only the Poppy grants go.
	Everything bool
}

// RemoveResult tells what was removed, what was kept and which grants remain.
type RemoveResult struct {
	URL             string   `json:"url"`
	DryRun          bool     `json:"dry_run"`
	Gone            bool     `json:"gone"`
	Removed         []string `json:"removed"`
	Kept            []string `json:"kept"`
	RemainingGrants []Grant  `json:"remaining_grants"`
}

// removalSteps lists the changes of a removal in the order they are made: the
// grants, the agent record and the agent app.
func (s *Service) removalSteps(removal *Removal, opts RemoveOptions) []step {
	everything := opts.Everything && !removal.Gone

	grants := removal.PoppyGrants
	if everything {
		grants = slices.Concat(grants, removal.OtherGrants)
	}

	var steps []step

	for _, grant := range grants {
		steps = append(steps, step{"delete", fmt.Sprintf("client grant %s (%s)", grant.ID, grant.Audience), func(ctx context.Context) error {
			return s.Grants.Delete(ctx, grant.ID)
		}})
	}

	if everything && removal.AgentID != "" {
		steps = append(steps, step{"delete", "agent record " + removal.AgentID, func(ctx context.Context) error {
			return s.Agents.Delete(ctx, removal.AgentID)
		}})
	}

	if everything {
		steps = append(steps, step{"delete", "agent app " + removal.ClientID, func(ctx context.Context) error {
			return s.Clients.Delete(ctx, removal.ClientID)
		}})
	}

	return steps
}

// outcome describes a removal that goes through all of its steps.
func outcome(removal *Removal, opts RemoveOptions, steps []step) *RemoveResult {
	result := &RemoveResult{
		URL:             removal.URL,
		Gone:            removal.Gone,
		Removed:         make([]string, len(steps)),
		Kept:            []string{},
		RemainingGrants: []Grant{},
	}

	for i, st := range steps {
		result.Removed[i] = st.noun
	}

	if !opts.Everything && !removal.Gone {
		result.RemainingGrants = removal.OtherGrants

		if removal.AgentID != "" {
			result.Kept = append(result.Kept, "agent record "+removal.AgentID)
		}
		result.Kept = append(result.Kept, "agent app "+removal.ClientID)
	}

	return result
}

// Plan reports what Remove would do, without changing anything.
func (s *Service) Plan(removal *Removal, opts RemoveOptions) *RemoveResult {
	result := outcome(removal, opts, s.removalSteps(removal, opts))
	result.DryRun = true

	return result
}

// Remove deletes the Poppy grants, then, with
// Everything, the other grants, the agent record and the agent app. It stops at
// the first failure; a failure after the first change returns a *PartialError.
func (s *Service) Remove(ctx context.Context, removal *Removal, opts RemoveOptions) (*RemoveResult, error) {
	steps := s.removalSteps(removal, opts)

	if err := runSteps(ctx, steps); err != nil {
		return nil, err
	}

	return outcome(removal, opts, steps), nil
}

// Approval says whether a removal may go ahead and what is needed first.
type Approval int

const (
	// Proceed means the removal can run now.
	Proceed Approval = iota
	// Ask means the user must confirm the plan.
	Ask
	// Choose means the user must pick between removing only the Poppy grants and
	// removing everything.
	Choose
	// Refuse means the removal needs confirmation but cannot ask for it.
	Refuse
)

// Decision is the outcome of Decide.
type Decision struct {
	Approval Approval
	// Everything is true when the removal also covers the other grants, the agent
	// record and the agent app. For Choose it is the answer when the user does not choose.
	Everything bool
}

// Decide applies the removal rules. An agent that is already gone, or force,
// goes ahead. Otherwise a terminal is needed to confirm, and without one the
// removal is refused. The other grants, the record and the app are removed only
// when asked for with removeAll, or when no other grant exists.
func Decide(removal *Removal, removeAll, force, interactive bool) Decision {
	everything := !removal.Gone && (removeAll || len(removal.OtherGrants) == 0)

	switch {
	case removal.Gone || force:
		return Decision{Approval: Proceed, Everything: everything}
	case !interactive:
		return Decision{Approval: Refuse, Everything: everything}
	case len(removal.OtherGrants) > 0 && !removeAll:
		return Decision{Approval: Choose}
	default:
		return Decision{Approval: Ask, Everything: everything}
	}
}

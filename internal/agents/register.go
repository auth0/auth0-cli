package agents

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/text"
)

// Registration is the state of an agent being registered.
type Registration struct {
	URL          string `json:"url"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	AppCreated   bool   `json:"app_created"`
	DPoPRequired bool   `json:"dpop_required"`
	AgentID      string `json:"agent_id"`
	AgentCreated bool   `json:"agent_created"`

	// Adopted is true when the agent record already existed.
	adopted bool
}

// newRegistration starts the registration of the agent in a checked metadata
// document. The name defaults to the document's name.
func newRegistration(preview *Preview, name string) *Registration {
	return &Registration{
		URL:        preview.URL,
		Name:       cmp.Or(name, preview.Name),
		AppCreated: preview.ClientID == "",
	}
}

// registerSteps lists the changes that register the agent, in the order they
// are made: it creates or refreshes the agent app, makes it require DPoP when
// asked to, creates the agent record, and reads the agent app back to report
// whether it requires DPoP and, for a record that already existed, to check
// that the two are linked. Each step records its result in reg. Run them with
// RunSteps, after any steps of the caller that must come first or last.
func (s *Service) registerSteps(reg *Registration, requireDPoP bool) []Step {
	steps := []Step{{"register", "agent app", s.registerApp(reg)}}

	if requireDPoP {
		steps = append(steps, Step{"require", "DPoP", s.requireDPoP(reg)})
	}

	return append(steps,
		Step{"create", "agent record", s.createRecord(reg)},
		Step{"read", "agent app", s.readApp(reg)},
	)
}

func (s *Service) registerApp(reg *Registration) func(context.Context) error {
	return func(ctx context.Context) error {
		resp, err := s.Clients.RegisterCimdClient(ctx, &managementv3.RegisterCimdClientRequestContent{ExternalClientID: reg.URL})
		if err != nil {
			return err
		}

		reg.ClientID = resp.GetClientID()

		return nil
	}
}

func (s *Service) requireDPoP(reg *Registration) func(context.Context) error {
	return func(ctx context.Context) error {
		required := true
		_, err := s.Clients.Update(ctx, reg.ClientID, &managementv3.UpdateClientRequestContent{RequireProofOfPossession: &required})

		return err
	}
}

// createRecord creates the agent record, or finds the existing one when the
// agent URL is already registered.
func (s *Service) createRecord(reg *Registration) func(context.Context) error {
	return func(ctx context.Context) error {
		record, err := s.Agents.Create(ctx, &managementv3.CreateAgentRequestContent{
			Name:            reg.Name,
			ClientID:        &reg.ClientID,
			ExternalAgentID: &reg.URL,
		})
		if err == nil {
			reg.AgentID, reg.AgentCreated = record.GetAgentID(), true

			return nil
		}

		if statusOf(err) != 409 {
			return err
		}

		records, listErr := s.Records(ctx)
		if listErr != nil {
			return listErr
		}

		i := slices.IndexFunc(records, func(record *managementv3.AgentResponseContent) bool {
			return record.GetExternalAgentID() == reg.URL
		})
		if i < 0 {
			return fmt.Errorf("agent %s is already registered, but its agent record was not found: %w", reg.URL, err)
		}

		reg.AgentID, reg.adopted = records[i].GetAgentID(), true

		return nil
	}
}

// LinkMismatchError reports an agent app that is linked to another agent record
// than the one that already exists for its URL.
type LinkMismatchError struct {
	URL      string
	ClientID string
	// LinkedAgentID is the agent record the app is linked to, RecordID the one
	// that exists for the URL.
	LinkedAgentID string
	RecordID      string
}

func (e *LinkMismatchError) Error() string {
	return fmt.Sprintf("the agent app %s is linked to agent %s, but the agent record for %s is %s", e.ClientID, e.LinkedAgentID, e.URL, e.RecordID)
}

// readApp reads the agent app back. Its DPoP setting is what the registration
// reports. A record that already existed is adopted rather than created, so the
// app is linked to it when it is not yet; an app linked to another record is left
// alone and reported.
func (s *Service) readApp(reg *Registration) func(context.Context) error {
	return func(ctx context.Context) error {
		app, err := s.FindApp(ctx, reg.URL)
		if err != nil {
			return err
		}

		if app == nil {
			return fmt.Errorf("no agent app was found for %s", reg.URL)
		}

		reg.DPoPRequired = app.GetRequireProofOfPossession()

		if !reg.adopted {
			return nil
		}

		switch linked := app.GetAgentID(); linked {
		case reg.AgentID:
			return nil
		case "":
			_, err := s.Clients.Update(ctx, reg.ClientID, &managementv3.UpdateClientRequestContent{AgentID: &reg.AgentID})

			return err
		default:
			return &LinkMismatchError{URL: reg.URL, ClientID: reg.ClientID, LinkedAgentID: linked, RecordID: reg.AgentID}
		}
	}
}

// ErrScopesRequired is returned when no scopes were chosen for the grants.
var ErrScopesRequired = errors.New("scopes are required")

// UnknownScopeError reports a scope that none of the APIs it applies to offers.
type UnknownScopeError struct {
	Scope string
	// APIs are the identifiers of the APIs that were checked for it.
	APIs []string
}

func (e *UnknownScopeError) Error() string {
	return fmt.Sprintf("scope %q is not defined on %s", e.Scope, strings.Join(e.APIs, ", "))
}

// NoScopesError reports an API that is left with no scope to grant.
type NoScopesError struct {
	API API
}

func (e *NoScopesError) Error() string {
	return "no scope is chosen for " + e.API.Identifier
}

// offers reports whether the API offers the scope.
func (a API) offers(name string) bool {
	return slices.ContainsFunc(a.Scopes, func(scope Scope) bool { return scope.Name == name })
}

// pinnedScope splits "<api>=<scope>", where the API is the ID or identifier of
// one of the APIs, from a scope name. It returns -1 for a name that is not
// pinned to an API.
func pinnedScope(apis []API, name string) (index int, scope string) {
	if i := strings.LastIndex(name, "="); i > 0 {
		ref := name[:i]
		if at := slices.IndexFunc(apis, func(api API) bool { return api.ID == ref || api.Identifier == ref }); at >= 0 {
			return at, name[i+1:]
		}
	}

	return -1, name
}

// ChooseScopes builds a grant for each API from the scopes named, or from every
// scope the API offers when all is set. A name grants the scope on each API that
// offers it; "<api>=<scope>" grants it on that API only, which tells apart a
// name that several APIs offer. A name is listed once per API. It returns
// ErrScopesRequired when neither names nor all is given, an *UnknownScopeError
// for a name that no API offers, and a *NoScopesError for an API left with no
// scope.
func ChooseScopes(apis []API, names []string, all bool) ([]Grant, error) {
	var given []string

	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			given = append(given, name)
		}
	}

	if !all && len(given) == 0 {
		return nil, ErrScopesRequired
	}

	chosen := make([][]string, len(apis))
	add := func(i int, scope string) {
		if !slices.Contains(chosen[i], scope) {
			chosen[i] = append(chosen[i], scope)
		}
	}

	if all {
		for i, api := range apis {
			for _, scope := range api.Scopes {
				add(i, scope.Name)
			}
		}
	}

	for _, name := range given {
		if i, scope := pinnedScope(apis, name); i >= 0 {
			if !apis[i].offers(scope) {
				return nil, &UnknownScopeError{Scope: scope, APIs: []string{apis[i].Identifier}}
			}

			add(i, scope)

			continue
		}

		var offered bool

		for i, api := range apis {
			if api.offers(name) {
				add(i, name)

				offered = true
			}
		}

		if !offered {
			identifiers := make([]string, len(apis))
			for i, api := range apis {
				identifiers[i] = api.Identifier
			}

			return nil, &UnknownScopeError{Scope: name, APIs: identifiers}
		}
	}

	grants := make([]Grant, len(apis))
	for i, api := range apis {
		if len(chosen[i]) == 0 {
			return nil, &NoScopesError{API: api}
		}

		grants[i] = Grant{Audience: api.Identifier, Scopes: chosen[i]}
	}

	return grants, nil
}

// RegisterRequest describes the agent to register.
type RegisterRequest struct {
	// Preview is the checked metadata document of the agent.
	Preview *Preview
	// Name is the name of the agent record. It defaults to the document's name.
	Name string
	// RequireDPoP makes the agent app require DPoP-bound tokens.
	RequireDPoP bool
	// Grants are the APIs and scopes to grant the agent app.
	Grants []Grant
}

// GrantResult is a grant of a registration. Created is false when the grant
// already existed and was left as it was. Note and Fix are set when the existing
// grant has other scopes than the ones requested.
type GrantResult struct {
	Grant
	Created bool   `json:"created"`
	Note    string `json:"note,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

// Registered is the final state of a registered agent.
type Registered struct {
	Registration
	Grants []GrantResult `json:"grants"`
}

// Register creates or refreshes the agent app, creates the agent record, and
// grants the agent app its APIs. It stops at the first failure; a failure after
// the first change returns a *PartialError. Running it again for the same agent
// changes nothing else. An agent record that already exists for the URL is used
// as it is, and linked to the agent app when it is not yet.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*Registered, error) {
	reg := &Registered{
		Registration: *newRegistration(req.Preview, req.Name),
		Grants:       []GrantResult{},
	}

	steps := s.registerSteps(&reg.Registration, req.RequireDPoP)

	for _, grant := range req.Grants {
		steps = append(steps, Step{Verb: "grant", Noun: grant.Audience, Run: s.grant(reg, grant)})
	}

	if err := RunSteps(ctx, steps); err != nil {
		return nil, err
	}

	return reg, nil
}

// grant gives the agent app access to an API, unless it already has a user
// grant for it. A grant of another subject type, such as the client grant of a
// machine-to-machine app, does not let the agent act for a person, so it does
// not count. An existing user grant is left as it is; when its scopes are not the
// ones requested, the result says so.
func (s *Service) grant(reg *Registered, grant Grant) func(context.Context) error {
	return func(ctx context.Context) error {
		page, err := s.Grants.List(ctx, &managementv3.ListClientGrantsRequestParameters{ClientID: &reg.ClientID, Audience: &grant.Audience})
		if err != nil {
			return err
		}

		existing, err := Collect(ctx, page)
		if err != nil {
			return err
		}

		if i := slices.IndexFunc(existing, func(g *managementv3.ClientGrantResponseContent) bool {
			return g.GetSubjectType() == managementv3.ClientGrantSubjectTypeEnumUser
		}); i >= 0 {
			result := GrantResult{Grant: Grant{ID: existing[i].GetID(), Audience: grant.Audience, Scopes: existing[i].GetScope()}}
			result.Note, result.Fix = scopeDifference(result.Grant, grant.Scopes, s.Tenant)
			reg.Grants = append(reg.Grants, result)

			return nil
		}

		subject := managementv3.ClientGrantSubjectTypeEnumUser
		created, err := s.Grants.Create(ctx, &managementv3.CreateClientGrantRequestContent{
			ClientID:    &reg.ClientID,
			Audience:    grant.Audience,
			Scope:       grant.Scopes,
			SubjectType: &subject,
		})
		if err != nil {
			return err
		}

		reg.Grants = append(reg.Grants, GrantResult{Grant: Grant{ID: created.GetID(), Audience: grant.Audience, Scopes: grant.Scopes}, Created: true})

		return nil
	}
}

// scopeDifference describes how the scopes of an existing grant differ from the
// requested ones, with the command that sets them. Both are empty when the sets
// are equal.
func scopeDifference(existing Grant, requested []string, tenant string) (note, fix string) {
	missing, extra := difference(requested, existing.Scopes), difference(existing.Scopes, requested)
	if len(missing) == 0 && len(extra) == 0 {
		return "", ""
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "lacks "+strings.Join(missing, ", "))
	}

	if len(extra) > 0 {
		parts = append(parts, "also has "+strings.Join(extra, ", "))
	}

	words := []string{"auth0", "client-grants", "update", text.Quote(existing.ID), "--scopes", text.Quote(strings.Join(requested, ","))}
	if len(requested) == 0 {
		words = []string{"auth0", "client-grants", "update", text.Quote(existing.ID), "--no-scopes"}
	}

	if tenant != "" {
		words = append(words, "--tenant", text.Quote(tenant))
	}

	return fmt.Sprintf("the existing grant for %s was left as it is and %s", existing.Audience, strings.Join(parts, " and ")),
		strings.Join(words, " ")
}

// difference returns the values of a that are not in b, sorted.
func difference(a, b []string) []string {
	var out []string

	for _, value := range a {
		if !slices.Contains(b, value) && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}

	slices.Sort(out)

	return out
}

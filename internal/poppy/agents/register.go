package agents

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// ErrScopesRequired is returned when no scopes were chosen for the grants.
var ErrScopesRequired = errors.New("scopes are required")

// ecKeysNote explains the key types Auth0 accepts for agent apps today.
const ecKeysNote = "Auth0 stores only RSA keys for agent apps today, so an agent with only EC keys (ES256) can't be registered yet."

// Key is a public key listed in an agent's metadata document.
type Key struct {
	KID string
	Alg string
}

// Preview is what Auth0 reads from an agent's metadata document.
type Preview struct {
	URL          string
	Name         string
	LogoURI      string
	RedirectURLs []string
	Keys         []Key
	Warnings     []string
	// ClientID is the agent app already registered for the URL, if any.
	ClientID string
}

// InvalidDocumentError reports an agent metadata document Auth0 rejects.
type InvalidDocumentError struct {
	URL        string
	Violations []string
}

// Error lists the violations, and explains the key types when they are the cause.
func (e *InvalidDocumentError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "the agent metadata document at %s is not valid:", e.URL)
	for _, violation := range e.Violations {
		b.WriteString("\n  ")
		b.WriteString(violation)
	}

	if slices.ContainsFunc(e.Violations, aboutKeys) {
		b.WriteString("\n")
		b.WriteString(ecKeysNote)
	}

	return b.String()
}

func aboutKeys(violation string) bool {
	violation = strings.ToLower(violation)

	return strings.Contains(violation, "key") || strings.Contains(violation, "jwk") || strings.Contains(violation, "alg")
}

// Preview asks Auth0 to fetch and check an agent's metadata document without
// creating anything. A document Auth0 rejects returns an *InvalidDocumentError.
func (s *Service) Preview(ctx context.Context, agentURL string) (*Preview, error) {
	resp, err := s.Clients.PreviewCimdMetadata(ctx, &managementv3.PreviewCimdMetadataRequestContent{ExternalClientID: agentURL})
	if err != nil {
		return nil, fmt.Errorf("failed to preview the agent metadata document: %w", err)
	}

	if errs := resp.GetErrors(); len(errs) > 0 {
		return nil, &InvalidDocumentError{URL: agentURL, Violations: errs}
	}

	if !resp.Validation.GetValid() {
		return nil, &InvalidDocumentError{URL: agentURL, Violations: resp.Validation.GetViolations()}
	}

	fields := resp.MappedFields
	preview := &Preview{
		URL:          agentURL,
		Name:         fields.GetName(),
		LogoURI:      fields.GetLogoURI(),
		RedirectURLs: fields.GetCallbacks(),
		Warnings:     resp.Validation.GetWarnings(),
		ClientID:     resp.GetClientID(),
	}

	if fields != nil && fields.ClientAuthenticationMethods != nil && fields.ClientAuthenticationMethods.PrivateKeyJwt != nil {
		for _, credential := range fields.ClientAuthenticationMethods.PrivateKeyJwt.Credentials {
			preview.Keys = append(preview.Keys, Key{KID: credential.Kid, Alg: credential.Alg})
		}
	}

	return preview, nil
}

// ChooseScopes builds a grant for each API from the scopes named, or from every
// scope the API offers when all is set. It returns ErrScopesRequired when
// neither is given.
func ChooseScopes(apis []API, names []string, all bool) ([]Grant, error) {
	if !all && len(names) == 0 {
		return nil, ErrScopesRequired
	}

	grants := make([]Grant, len(apis))
	for i, api := range apis {
		scopes := make([]string, 0, len(api.Scopes))

		if all {
			for _, scope := range api.Scopes {
				scopes = append(scopes, scope.Name)
			}
		}

		for _, name := range names {
			if !slices.ContainsFunc(api.Scopes, func(scope Scope) bool { return scope.Name == name }) {
				return nil, fmt.Errorf("scope %q is not defined on %s", name, api.Identifier)
			}
			scopes = append(scopes, name)
		}

		grants[i] = Grant{Audience: api.Identifier, Scopes: scopes}
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

// Registration is the final state of a registered agent.
type Registration struct {
	URL          string        `json:"url"`
	Name         string        `json:"name"`
	ClientID     string        `json:"client_id"`
	AppCreated   bool          `json:"app_created"`
	DPoPRequired bool          `json:"dpop_required"`
	AgentID      string        `json:"agent_id"`
	AgentCreated bool          `json:"agent_created"`
	Grants       []GrantResult `json:"grants"`
}

// Register creates or refreshes the agent app, creates the agent record,
// and grants the agent app its APIs. It stops
// at the first failure; a failure after the first change returns a
// *PartialError. Running it again for the same agent changes nothing else.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*Registration, error) {
	reg := &Registration{
		URL:        req.Preview.URL,
		Name:       cmp.Or(req.Name, req.Preview.Name),
		AppCreated: req.Preview.ClientID == "",
		Grants:     []GrantResult{},
	}

	steps := []step{{"register", "agent app", s.registerApp(reg)}}

	if req.RequireDPoP {
		steps = append(steps, step{"require", "DPoP", s.requireDPoP(reg)})
	}

	steps = append(steps, step{"create", "agent record", s.createRecord(reg)})

	for _, grant := range req.Grants {
		steps = append(steps, step{"grant", grant.Audience, s.grant(reg, grant)})
	}

	if err := runSteps(ctx, steps); err != nil {
		return nil, err
	}

	return reg, nil
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
		if _, err := s.Clients.Update(ctx, reg.ClientID, &managementv3.UpdateClientRequestContent{RequireProofOfPossession: &required}); err != nil {
			return err
		}

		reg.DPoPRequired = true

		return nil
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

		records, listErr := s.listRecords(ctx)
		if listErr != nil {
			return listErr
		}

		i := slices.IndexFunc(records, func(record *managementv3.AgentResponseContent) bool {
			return record.GetExternalAgentID() == reg.URL
		})
		if i < 0 {
			return fmt.Errorf("%s is already registered, but its agent record was not found: %w", reg.URL, err)
		}

		reg.AgentID = records[i].GetAgentID()

		return nil
	}
}

// grant gives the agent app access to an API, unless it already has a user
// grant for it. A grant of another subject type, such as the client grant of a
// machine-to-machine app, does not let the agent act for a person, so it does
// not count. An existing user grant is left as it is; when its scopes are not the
// ones requested, the result says so.
func (s *Service) grant(reg *Registration, grant Grant) func(context.Context) error {
	return func(ctx context.Context) error {
		page, err := s.Grants.List(ctx, &managementv3.ListClientGrantsRequestParameters{ClientID: &reg.ClientID, Audience: &grant.Audience})
		if err != nil {
			return err
		}

		existing, err := collect(ctx, page)
		if err != nil {
			return err
		}

		if i := slices.IndexFunc(existing, func(g *managementv3.ClientGrantResponseContent) bool {
			return g.GetSubjectType() == managementv3.ClientGrantSubjectTypeEnumUser
		}); i >= 0 {
			result := GrantResult{Grant: Grant{ID: existing[i].GetID(), Audience: grant.Audience, Scopes: existing[i].GetScope()}}
			result.Note, result.Fix = scopeDifference(result.Grant, grant.Scopes)
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
func scopeDifference(existing Grant, requested []string) (note, fix string) {
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

	scopes, _ := json.Marshal(requested)

	return fmt.Sprintf("the existing grant for %s was left as it is and %s", existing.Audience, strings.Join(parts, " and ")),
		fmt.Sprintf("auth0 api patch client-grants/%s --data '{\"scope\":%s}'", existing.ID, scopes)
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

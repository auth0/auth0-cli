// Package agents implements the Poppy agent commands (register, list, show and
// remove) on top of narrow interfaces over the Management API. It has no
// dependency on Cobra, so every flow can be tested with mocks.
package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// ErrNoPoppyAPI is returned when the tenant has no API with the Personal Agent
// Protocol on.
var ErrNoPoppyAPI = errors.New("no Poppy API found")

// unknown is what a value reads as when the call that provides it failed.
const unknown = "unknown"

// clientFields limits the clients the commands read to what they use.
const clientFields = "client_id,name,agent_id,external_client_id,external_metadata_type,external_metadata_status,require_proof_of_possession"

// ClientAPI is the part of the clients API the agent commands use.
type ClientAPI interface {
	List(ctx context.Context, request *managementv3.ListClientsRequestParameters, opts ...option.RequestOption) (*auth0.ClientListPage, error)
	Update(ctx context.Context, id string, request *managementv3.UpdateClientRequestContent, opts ...option.RequestOption) (*managementv3.UpdateClientResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
	PreviewCimdMetadata(ctx context.Context, request *managementv3.PreviewCimdMetadataRequestContent, opts ...option.RequestOption) (*managementv3.PreviewCimdMetadataResponseContent, error)
	RegisterCimdClient(ctx context.Context, request *managementv3.RegisterCimdClientRequestContent, opts ...option.RequestOption) (*managementv3.RegisterCimdClientResponseContent, error)
}

// AgentAPI is the part of the agents API the agent commands use.
type AgentAPI interface {
	List(ctx context.Context, request *managementv3.ListAgentsRequestParameters, opts ...option.RequestOption) (*auth0.AgentPage, error)
	Create(ctx context.Context, request *managementv3.CreateAgentRequestContent, opts ...option.RequestOption) (*managementv3.AgentResponseContent, error)
	Read(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.AgentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

// GrantAPI is the part of the client grants API the agent commands use.
type GrantAPI interface {
	List(ctx context.Context, request *managementv3.ListClientGrantsRequestParameters, opts ...option.RequestOption) (*auth0.ClientGrantPage, error)
	Create(ctx context.Context, request *managementv3.CreateClientGrantRequestContent, opts ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

// ResourceServerAPI is the part of the resource servers API the agent commands use.
type ResourceServerAPI interface {
	List(ctx context.Context, request *managementv3.ListResourceServerRequestParameters, opts ...option.RequestOption) (*auth0.ResourceServerPage, error)
}

// CredentialAPI is the part of the client credentials API the agent commands use.
type CredentialAPI interface {
	List(ctx context.Context, clientID string, opts ...option.RequestOption) ([]*managementv3.ClientCredential, error)
}

// Service runs the agent commands against the Management API.
type Service struct {
	Clients         ClientAPI
	Agents          AgentAPI
	Grants          GrantAPI
	ResourceServers ResourceServerAPI
	Credentials     CredentialAPI
}

// Maybe is a value that may be unknown. An unknown value is encoded as the
// string "unknown", so it is never omitted from the JSON output.
type Maybe[T any] struct {
	Value T
	Known bool
}

// MarshalJSON encodes the value, or "unknown" when it is not known.
func (m Maybe[T]) MarshalJSON() ([]byte, error) {
	if !m.Known {
		return json.Marshal(unknown)
	}

	return json.Marshal(m.Value)
}

func known[T any](value T) Maybe[T] {
	return Maybe[T]{Value: value, Known: true}
}

// Grant is the access an agent app has to an API.
type Grant struct {
	ID       string   `json:"id,omitempty"`
	Audience string   `json:"audience"`
	Scopes   []string `json:"scopes"`
	// SubjectType is the subject type of the client grant, such as user or
	// client. Only a user grant lets the agent act for a person.
	SubjectType string `json:"-"`
}

// userSubject is the subject type of a client grant that lets an agent act for
// a person.
const userSubject = string(managementv3.ClientGrantSubjectTypeEnumUser)

// IsUserGrant tells whether the grant is for users, the only kind that gives an
// agent access to an API.
func (g Grant) IsUserGrant() bool {
	return g.SubjectType == userSubject
}

// API is a Poppy API: a resource server with the Personal Agent Protocol on.
type API struct {
	ID         string  `json:"id"`
	Identifier string  `json:"identifier"`
	Scopes     []Scope `json:"scopes"`
}

// Scope is a scope an API offers.
type Scope struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PartialError reports a command that stopped after changing some things.
type PartialError struct {
	Applied   []string
	Remaining []string
	Err       error
}

// Error lists what was done and what is left.
func (e *PartialError) Error() string {
	return fmt.Sprintf("stopped partway: %v\n  applied: %s\n  remaining: %s",
		e.Err, strings.Join(e.Applied, ", "), strings.Join(e.Remaining, ", "))
}

// Unwrap returns the error that stopped the command.
func (e *PartialError) Unwrap() error { return e.Err }

// ErrorDetails gives the JSON error output the applied and remaining steps.
func (e *PartialError) ErrorDetails() json.RawMessage {
	details, _ := json.Marshal(map[string]any{"code": "partial", "applied": e.Applied, "remaining": e.Remaining})

	return details
}

// step is one change a command makes. It reads as "<verb> <noun>".
type step struct {
	verb string
	noun string
	run  func(ctx context.Context) error
}

func (s step) String() string { return s.verb + " " + s.noun }

// runSteps runs the steps in order and stops at the first failure. A failure
// after the first step is reported as a *PartialError.
func runSteps(ctx context.Context, steps []step) error {
	for i, s := range steps {
		if err := s.run(ctx); err != nil {
			err = fmt.Errorf("failed to %s: %w", s, err)
			if i == 0 {
				return err
			}

			return &PartialError{Applied: stepStrings(steps[:i]), Remaining: stepStrings(steps[i:]), Err: err}
		}
	}

	return nil
}

func stepStrings(steps []step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.String()
	}

	return out
}

// collect reads every page of a list response.
func collect[C comparable, T, R any](ctx context.Context, page *core.Page[C, T, R]) ([]T, error) {
	var items []T

	iter := page.Iterator()
	for iter.Next(ctx) {
		items = append(items, iter.Current())
	}

	return items, iter.Err()
}

// statusOf returns the HTTP status carried by a Management API error, or 0.
func statusOf(err error) int {
	var apiErr *core.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}

	return 0
}

// isURL reports whether ref is an agent URL rather than an agent ID.
func isURL(ref string) bool {
	return strings.HasPrefix(ref, "https://")
}

// poppyAPIs returns every API that has the Personal Agent Protocol on.
func (s *Service) poppyAPIs(ctx context.Context) ([]API, error) {
	page, err := s.ResourceServers.List(ctx, &managementv3.ListResourceServerRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}

	servers, err := collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}

	var apis []API
	for _, server := range servers {
		if !server.GetUseForPersonalAgentProtocol() {
			continue
		}

		api := API{ID: server.GetID(), Identifier: server.GetIdentifier()}
		for _, scope := range server.GetScopes() {
			api.Scopes = append(api.Scopes, Scope{Name: scope.GetValue(), Description: scope.GetDescription()})
		}
		apis = append(apis, api)
	}

	return apis, nil
}

// PoppyAPIs returns the Poppy APIs picked by ID or identifier, or every Poppy
// API when none are picked.
func (s *Service) PoppyAPIs(ctx context.Context, picked []string) ([]API, error) {
	all, err := s.poppyAPIs(ctx)
	if err != nil {
		return nil, err
	}

	if len(picked) == 0 {
		if len(all) == 0 {
			return nil, fmt.Errorf("%w. Run: auth0 poppy setup", ErrNoPoppyAPI)
		}

		return all, nil
	}

	var apis []API
	for _, pick := range picked {
		i := slices.IndexFunc(all, func(api API) bool { return api.ID == pick || api.Identifier == pick })
		if i < 0 {
			return nil, fmt.Errorf("%q is not a Poppy API", pick)
		}

		if !slices.ContainsFunc(apis, func(api API) bool { return api.ID == all[i].ID }) {
			apis = append(apis, all[i])
		}
	}

	return apis, nil
}

// grantsOf returns every client grant of an app.
func (s *Service) grantsOf(ctx context.Context, clientID string) ([]Grant, error) {
	page, err := s.Grants.List(ctx, &managementv3.ListClientGrantsRequestParameters{ClientID: &clientID})
	if err != nil {
		return nil, err
	}

	grants, err := collect(ctx, page)
	if err != nil {
		return nil, err
	}

	out := make([]Grant, len(grants))
	for i, grant := range grants {
		out[i] = Grant{ID: grant.GetID(), Audience: grant.GetAudience(), Scopes: grant.GetScope(), SubjectType: string(grant.GetSubjectType())}
	}

	return out, nil
}

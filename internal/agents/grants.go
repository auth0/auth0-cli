package agents

import (
	"context"
	"fmt"
	"slices"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/auth0"
)

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

// API is an API of the tenant: a resource server and the scopes it offers.
type API struct {
	ID         string  `json:"id"`
	Name       string  `json:"name,omitempty"`
	Identifier string  `json:"identifier"`
	Scopes     []Scope `json:"scopes"`
}

// Scope is a scope an API offers.
type Scope struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// NewAPI describes a resource server as an API.
func NewAPI(server *managementv3.ResourceServer) API {
	api := API{ID: server.GetID(), Name: server.GetName(), Identifier: server.GetIdentifier()}
	for _, scope := range server.GetScopes() {
		api.Scopes = append(api.Scopes, Scope{Name: scope.GetValue(), Description: scope.GetDescription()})
	}

	return api
}

// ListServers returns every resource server of the tenant.
func (s *Service) ListServers(ctx context.Context) ([]*managementv3.ResourceServer, error) {
	page, err := s.ResourceServers.List(ctx, &managementv3.ListResourceServerRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}

	servers, err := Collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}

	return servers, nil
}

// APIs returns every API of the tenant.
func (s *Service) APIs(ctx context.Context) ([]API, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	apis := make([]API, len(servers))
	for i, server := range servers {
		apis[i] = NewAPI(server)
	}

	return apis, nil
}

// UnknownAPIError reports an API that is not among the APIs on offer.
type UnknownAPIError struct {
	// Ref is the ID or identifier that was given.
	Ref string
}

func (e *UnknownAPIError) Error() string {
	return fmt.Sprintf("%q is not an API of this tenant", e.Ref)
}

// PickAPIs returns the APIs among all that are picked by ID or identifier, each
// once, in the order they were picked. It returns an *UnknownAPIError for a pick
// that matches none.
func PickAPIs(all []API, picked []string) ([]API, error) {
	var apis []API

	for _, pick := range picked {
		i := slices.IndexFunc(all, func(api API) bool { return api.ID == pick || api.Identifier == pick })
		if i < 0 {
			return nil, &UnknownAPIError{Ref: pick}
		}

		if !slices.ContainsFunc(apis, func(api API) bool { return api.ID == all[i].ID }) {
			apis = append(apis, all[i])
		}
	}

	return apis, nil
}

// newGrant describes a client grant.
func newGrant(grant *managementv3.ClientGrantResponseContent) Grant {
	return Grant{ID: grant.GetID(), Audience: grant.GetAudience(), Scopes: grant.GetScope(), SubjectType: string(grant.GetSubjectType())}
}

// GrantsOf returns every client grant of an app. The client grants API filters
// by client, but a removal deletes what this returns, so a grant of another
// client is dropped here instead of trusting the filter.
func (s *Service) GrantsOf(ctx context.Context, clientID string) ([]Grant, error) {
	page, err := s.Grants.List(ctx, &managementv3.ListClientGrantsRequestParameters{ClientID: &clientID})
	if err != nil {
		return nil, err
	}

	grants, err := Collect(ctx, page)
	if err != nil {
		return nil, err
	}

	out := make([]Grant, 0, len(grants))
	for _, grant := range grants {
		if grant.GetClientID() == clientID {
			out = append(out, newGrant(grant))
		}
	}

	return out, nil
}

// GrantsByClient returns the client grants of the tenant, grouped by the ID of
// the client they belong to. It makes one listing, however many clients there are.
func (s *Service) GrantsByClient(ctx context.Context) (map[string][]Grant, error) {
	page, err := s.Grants.List(ctx, &managementv3.ListClientGrantsRequestParameters{})
	if err != nil {
		return nil, err
	}

	grants, err := Collect(ctx, page)
	if err != nil {
		return nil, err
	}

	byClient := make(map[string][]Grant)
	for _, grant := range grants {
		byClient[grant.GetClientID()] = append(byClient[grant.GetClientID()], newGrant(grant))
	}

	return byClient, nil
}

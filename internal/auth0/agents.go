//go:generate go tool mockgen -source=agents.go -destination=mock/agents_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// AgentListPage aliases the checkpoint-paginated agents list response so mockgen
// can parse the interface (it cannot parse the multi-type-parameter generic inline).
type AgentListPage = core.Page[*string, *management.AgentResponseContent, *management.ListAgentsResponseContent]

// AgentsAPI describes the interface for agent operations.
type AgentsAPI interface {
	List(ctx context.Context, request *management.ListAgentsRequestParameters, opts ...option.RequestOption) (*AgentListPage, error)
	Create(ctx context.Context, request *management.CreateAgentRequestContent, opts ...option.RequestOption) (*management.AgentResponseContent, error)
	Read(ctx context.Context, id string, opts ...option.RequestOption) (*management.AgentResponseContent, error)
	Update(ctx context.Context, id string, request *management.PatchAgentRequestParameters, opts ...option.RequestOption) (*management.AgentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

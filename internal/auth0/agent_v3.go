//go:generate go tool mockgen -source=agent_v3.go -destination=mock/agent_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// AgentPage is the checkpoint-paginated response returned by the agents list
// endpoint. It is aliased here so the mock generator (which cannot parse
// instantiated generic types) sees a plain named type in the interface.
type AgentPage = core.Page[*string, *managementv3.AgentResponseContent, *managementv3.ListAgentsResponseContent]

// AgentAPIV3 is the V3 SDK interface for the /agents endpoint.
type AgentAPIV3 interface {
	// List agents.
	//
	// Required scope: `read:agents`.
	List(
		ctx context.Context,
		request *managementv3.ListAgentsRequestParameters,
		opts ...option.RequestOption,
	) (*AgentPage, error)

	// Create an agent.
	//
	// Required scope: `create:agents`.
	Create(
		ctx context.Context,
		request *managementv3.CreateAgentRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.AgentResponseContent, error)

	// Read a single agent.
	//
	// Required scope: `read:agents`.
	Read(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) (*managementv3.AgentResponseContent, error)

	// Delete an agent.
	//
	// Required scope: `delete:agents`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

//go:generate mockgen -source=flow_v3.go -destination=mock/flow_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// FlowSummaryPage aliases the paginated flows list response. The alias keeps the
// interface return type a single identifier so mockgen's source parser can handle
// it (it cannot parse the multi-type-parameter generic inline).
type FlowSummaryPage = core.Page[*int, *managementv3.FlowSummary, *managementv3.ListFlowsOffsetPaginatedResponseContent]

// FlowExecutionSummaryPage aliases the paginated flow-executions list response.
type FlowExecutionSummaryPage = core.Page[*string, *managementv3.FlowExecutionSummary, *managementv3.ListFlowExecutionsPaginatedResponseContent]

// FlowsVaultConnectionSummaryPage aliases the paginated vault-connections list response.
type FlowsVaultConnectionSummaryPage = core.Page[*int, *managementv3.FlowsVaultConnectionSummary, *managementv3.ListFlowsVaultConnectionsOffsetPaginatedResponseContent]

// FlowAPIV3 is the V3 SDK interface for the /flows endpoint. Create, read, and
// update go through the raw HTTP client to preserve the flow action graph that
// the typed request models would drop, so only paging and delete live here.
type FlowAPIV3 interface {
	// List flows.
	//
	// Required scope: `read:flows`.
	List(
		ctx context.Context,
		request *managementv3.ListFlowsRequestParameters,
		opts ...option.RequestOption,
	) (*FlowSummaryPage, error)

	// Delete a flow.
	//
	// Required scope: `delete:flows`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

// FlowExecutionAPIV3 is the V3 SDK interface for the /flows/{id}/executions
// endpoint. Executions are runtime-produced, so the surface is read and delete
// only.
type FlowExecutionAPIV3 interface {
	// List flow executions.
	//
	// Required scope: `read:flows_executions`.
	List(
		ctx context.Context,
		flowID string,
		request *managementv3.ListFlowExecutionsRequestParameters,
		opts ...option.RequestOption,
	) (*FlowExecutionSummaryPage, error)

	// Delete a flow execution.
	//
	// Required scope: `delete:flows_executions`.
	Delete(
		ctx context.Context,
		flowID string,
		executionID string,
		opts ...option.RequestOption,
	) error
}

// FlowVaultConnectionAPIV3 is the V3 SDK interface for the
// /flows/vault/connections endpoint. Create and update go through the raw HTTP
// client because the typed request model is a large per-provider union, so only
// paging and delete live here.
type FlowVaultConnectionAPIV3 interface {
	// List vault connections.
	//
	// Required scope: `read:flows_vault_connections`.
	List(
		ctx context.Context,
		request *managementv3.ListFlowsVaultConnectionsRequestParameters,
		opts ...option.RequestOption,
	) (*FlowsVaultConnectionSummaryPage, error)

	// Delete a vault connection.
	//
	// Required scope: `delete:flows_vault_connections`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

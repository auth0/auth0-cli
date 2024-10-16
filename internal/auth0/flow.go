//go:generate mockgen -source=flow.go -destination=mock/flow_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type FlowAPI interface {
	// Create a new flow.
	Create(ctx context.Context, r *management.Flow, opts ...management.RequestOption) error

	// Read flow details.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (r *management.Flow, err error)

	// Update an existing flow.
	Update(ctx context.Context, id string, r *management.Flow, opts ...management.RequestOption) error

	// Delete a flow.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// List flow.
	List(ctx context.Context, opts ...management.RequestOption) (r *management.FlowList, err error)
}

type FlowVaultConnectionAPI interface {
	// CreateConnection Create a new flow vault connection.
	CreateConnection(ctx context.Context, r *management.FlowVaultConnection, opts ...management.RequestOption) error

	// GetConnection Retrieve flow vault connection details.
	GetConnection(ctx context.Context, id string, opts ...management.RequestOption) (r *management.FlowVaultConnection, err error)

	// UpdateConnection Update an existing flow vault connection.
	UpdateConnection(ctx context.Context, id string, r *management.FlowVaultConnection, opts ...management.RequestOption) error

	// DeleteConnection Delete a flow vault connection.
	DeleteConnection(ctx context.Context, id string, opts ...management.RequestOption) error

	// GetConnectionList List flow vault connections.
	GetConnectionList(ctx context.Context, opts ...management.RequestOption) (r *management.FlowVaultConnectionList, err error)
}

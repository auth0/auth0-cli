//go:generate mockgen -source=action_module.go -destination=mock/action_module_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ActionModulePage is the paginated response returned by the action modules
// list endpoint. It is aliased here so the mock generator (which cannot parse
// instantiated generic types) sees a plain named type in the interface.
type ActionModulePage = core.Page[*int, *managementv3.ActionModuleListItem, *managementv3.GetActionModulesResponseContent]

// ActionModuleActionPage is the paginated response returned by the action
// module actions list endpoint. It is aliased here for the same reason as
// ActionModulePage.
type ActionModuleActionPage = core.Page[*int, *managementv3.ActionModuleAction, *managementv3.GetActionModuleActionsResponseContent]

// ActionModuleVersionPage is the paginated response returned by the action
// module versions list endpoint. It is aliased here for the same reason as
// ActionModulePage.
type ActionModuleVersionPage = core.Page[*int, *managementv3.ActionModuleVersion, *managementv3.GetActionModuleVersionsResponseContent]

// ActionModuleAPIV3 is the V3 SDK interface for the /actions/modules endpoint.
// Action modules are reusable code libraries that actions can import; they are
// keyed by module ID.
type ActionModuleAPIV3 interface {
	// List retrieves the action modules for the tenant. The results are
	// offset-paginated via the Page/PerPage request parameters.
	//
	// Required scope: `read:actions`.
	List(ctx context.Context, request *managementv3.GetActionModulesRequestParameters, opts ...option.RequestOption) (*ActionModulePage, error)

	// Get retrieves an action module by ID.
	//
	// Required scope: `read:actions`.
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.GetActionModuleResponseContent, error)

	// Create a new action module.
	//
	// Required scope: `create:actions`.
	Create(ctx context.Context, request *managementv3.CreateActionModuleRequestContent, opts ...option.RequestOption) (*managementv3.CreateActionModuleResponseContent, error)

	// Update an action module by ID. The module name is immutable and cannot be
	// changed through this endpoint.
	//
	// Required scope: `update:actions`.
	Update(ctx context.Context, id string, request *managementv3.UpdateActionModuleRequestContent, opts ...option.RequestOption) (*managementv3.UpdateActionModuleResponseContent, error)

	// Delete an action module by ID.
	//
	// Required scope: `delete:actions`.
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error

	// ListActions retrieves the actions that use an action module. The results
	// are offset-paginated via the Page/PerPage request parameters.
	//
	// Required scope: `read:actions`.
	ListActions(ctx context.Context, id string, request *managementv3.GetActionModuleActionsRequestParameters, opts ...option.RequestOption) (*ActionModuleActionPage, error)

	// Rollback copies the code, dependencies, and secrets of a past version
	// back into the module's draft.
	//
	// Required scope: `update:actions`.
	Rollback(ctx context.Context, id string, request *managementv3.RollbackActionModuleRequestParameters, opts ...option.RequestOption) (*managementv3.RollbackActionModuleResponseContent, error)
}

// ActionModuleVersionAPIV3 is the V3 SDK interface for the
// /actions/modules/{id}/versions endpoint. Publishing a module creates a new
// immutable version from its current draft.
type ActionModuleVersionAPIV3 interface {
	// Create publishes the module's current draft as a new immutable version.
	//
	// Required scope: `update:actions`.
	Create(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.CreateActionModuleVersionResponseContent, error)

	// List retrieves the immutable versions of an action module. The results
	// are offset-paginated via the Page/PerPage request parameters.
	//
	// Required scope: `read:actions`.
	List(ctx context.Context, id string, request *managementv3.GetActionModuleVersionsRequestParameters, opts ...option.RequestOption) (*ActionModuleVersionPage, error)

	// Get retrieves a specific immutable version of an action module.
	//
	// Required scope: `read:actions`.
	Get(ctx context.Context, id string, versionID string, opts ...option.RequestOption) (*managementv3.GetActionModuleVersionResponseContent, error)
}

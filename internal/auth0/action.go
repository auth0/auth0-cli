//go:generate mockgen -source=action.go -destination=mock/action_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type (
	ActionAPI interface {
		// Create a new action.
		//
		// See: https://auth0.com/docs/api/management/v2#!/Actions/post_action
		Create(ctx context.Context, a *management.Action, opts ...management.RequestOption) error

		// Read action details.
		//
		// See: https://auth0.com/docs/api/management/v2#!/Actions/get_action
		Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.Action, error)

		// Update an existing action.
		//
		// See: https://auth0.com/docs/api/management/v2#!/Actions/patch_action
		Update(ctx context.Context, id string, a *management.Action, opts ...management.RequestOption) error

		// Delete an action
		//
		// See: https://auth0.com/docs/api/management/v2#!/Actions/delete_action
		Delete(ctx context.Context, id string, opts ...management.RequestOption) error

		// List all actions.
		//
		// See: https://auth0.com/docs/api/management/v2#!/Actions/get_actions
		List(ctx context.Context, opts ...management.RequestOption) (c *management.ActionList, err error)

		// Triggers available.
		//
		// https://auth0.com/docs/api/management/v2/#!/Actions/get_triggers
		Triggers(ctx context.Context, opts ...management.RequestOption) (l *management.ActionTriggerList, err error)

		// Bindings lists the bindings of a trigger.
		//
		// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_bindings
		Bindings(ctx context.Context, triggerID string, opts ...management.RequestOption) (bl *management.ActionBindingList, err error)

		// UpdateBindings of a trigger.
		//
		// See: https://auth0.com/docs/api/management/v2/#!/Actions/patch_bindings
		UpdateBindings(ctx context.Context, triggerID string, bl []*management.ActionBinding, opts ...management.RequestOption) error

		// Deploy an action.
		//
		// See: https://auth0.com/docs/api/management/v2/#!/Actions/post_deploy_action
		Deploy(ctx context.Context, id string, opts ...management.RequestOption) (v *management.ActionVersion, err error)

		// Versions lists versions of an action.
		//
		// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_action_versions
		Versions(ctx context.Context, id string, opts ...management.RequestOption) (c *management.ActionVersionList, err error)
	}
)

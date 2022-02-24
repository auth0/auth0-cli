package auth0

import "github.com/auth0/go-auth0/management"

type ActionAPI interface {
	// Create a new action.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Actions/post_action
	Create(a *management.Action, opts ...management.RequestOption) error

	// Read action details.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Actions/get_action
	Read(id string, opts ...management.RequestOption) (*management.Action, error)

	// Update an existing action.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Actions/patch_action
	Update(id string, a *management.Action, opts ...management.RequestOption) error

	// Delete an action
	//
	// See: https://auth0.com/docs/api/management/v2#!/Actions/delete_action
	Delete(id string, opts ...management.RequestOption) error

	// List all actions.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Actions/get_actions
	List(opts ...management.RequestOption) (c *management.ActionList, err error)

	// Triggers available.
	//
	// https://auth0.com/docs/api/management/v2/#!/Actions/get_triggers
	Triggers(opts ...management.RequestOption) (l *management.ActionTriggerList, err error)

	// Deploy an action.
	//
	// See: https://auth0.com/docs/api/management/v2/#!/Actions/post_deploy_action
	Deploy(id string, opts ...management.RequestOption) (v *management.ActionVersion, err error)
}

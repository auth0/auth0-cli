//go:generate mockgen -source=actions.go -destination=actions_mock.go -package=auth0
package auth0

import "gopkg.in/auth0.v5/management"

type ActionAPI interface {
	Create(a *management.Action) error
	Read(id string) (*management.Action, error)
	Update(id string, a *management.Action) error
	Delete(id string, opts ...management.RequestOption) error
	List(opts ...management.RequestOption) (c *management.ActionList, err error)
}

type ActionVersionAPI interface {
	Create(actionID string, v *management.ActionVersion) error
	Read(actionID string, id string) (*management.ActionVersion, error)
	Update(id string, a *management.ActionVersion) error
	Delete(actionID string, id string, opts ...management.RequestOption) error
	Test(actionID string, id string, payload management.Object) (management.Object, error)
}

type ActionBindingAPI interface {
	List(triggerID management.TriggerID, opts ...management.RequestOption) (c *management.ActionBindingList, err error)
	Update(triggerID management.TriggerID, v *management.ActionBindingList) error
}

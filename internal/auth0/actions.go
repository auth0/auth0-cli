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

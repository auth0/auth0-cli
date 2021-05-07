//go:generate mockgen -source=custom_domain.go -destination=custom_domain_mock.go -package=auth0
package auth0

import "gopkg.in/auth0.v5/management"

type CustomDomainAPI interface {
	List(opts ...management.RequestOption) (c []*management.CustomDomain, err error)
}

//go:generate mockgen -source=rule.go -destination=rule_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type RuleAPI interface {
	// Create a new rule.
	//
	// Note: Changing a rule's stage of execution from the default `login_success`
	// can change the rule's function signature to have user omitted.
	Create(r *management.Rule, opts ...management.RequestOption) error

	// Retrieve rule details. Accepts a list of fields to include or exclude in the result.
	Read(id string, opts ...management.RequestOption) (r *management.Rule, err error)

	// Update an existing rule.
	Update(id string, r *management.Rule, opts ...management.RequestOption) error

	// Delete a rule.
	Delete(id string, opts ...management.RequestOption) error

	// List all rules.
	List(opts ...management.RequestOption) (r *management.RuleList, err error)
}

//go:generate mockgen -source=form.go -destination=form/form_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type FormAPI interface {
	// Create a new form.
	Create(ctx context.Context, r *management.Form, opts ...management.RequestOption) error

	// Read form details.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (r *management.Form, err error)

	// Update an existing action.
	Update(ctx context.Context, id string, r *management.Form, opts ...management.RequestOption) error

	// Delete an action.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// List form.
	List(ctx context.Context, opts ...management.RequestOption) (r *management.FormList, err error)
}

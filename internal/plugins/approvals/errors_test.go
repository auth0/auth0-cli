package approvals_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/plugins/approvals"
)

func TestNotApprovedError(t *testing.T) {
	tests := []struct {
		name string
		err  approvals.NotApprovedError
		want string
	}{
		{
			name: "never approved",
			err:  approvals.NotApprovedError{Tenant: "acme.us.auth0.com", ID: "auth0/checkmate"},
			want: "checkmate is not approved for acme.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com",
		},
		{
			name: "approved with other permissions",
			err:  approvals.NotApprovedError{Tenant: "acme.us.auth0.com", ID: "auth0/checkmate", Changed: true},
			want: "checkmate changed what it asks for since you approved it for acme.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.EqualError(t, &test.err, test.want)
		})
	}
}

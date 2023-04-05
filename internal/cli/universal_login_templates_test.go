package cli

import (
	"context"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

type mockManagamentError struct {
	error
	status int
}

func (m mockManagamentError) Status() int {
	return m.status
}

func TestEnsureCustomDomainIsEnabled(t *testing.T) {
	tests := []struct {
		name          string
		customDomains []*management.CustomDomain
		apiError      management.Error
		assertOutput  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			customDomains: []*management.CustomDomain{
				{
					Status:   auth0.String("foo"),
				},
				{
					Status:   auth0.String("ready"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "no verified domains",
			customDomains: []*management.CustomDomain{
				{
					Status:   auth0.String("foo"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name: "custom domains are not enabled",
			apiError: mockManagamentError{status: http.StatusForbidden},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name: "api error",
			apiError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
			customDomainAPI.EXPECT().
				List(gomock.Any()).
				Return(test.customDomains, test.apiError)

			ctx := context.Background()
			api := &auth0.API{CustomDomain: customDomainAPI}
			err := ensureCustomDomainIsEnabled(ctx, api)
			test.assertOutput(t, err)
		})
	}
}

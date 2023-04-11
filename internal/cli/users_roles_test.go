package cli

import (
	"errors"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestUserRolesToRemovePickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		roles        []*management.Role
		apiError     error
		assertOutput func(t testing.TB, options []string)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			userID: "",
			roles: []*management.Role{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options []string) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-id-1 (Name: some-name-1)", options[0])
				assert.Equal(t, "some-id-2 (Name: some-name-2)", options[1])
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:  "no roles for user",
			userID: "",
			roles: []*management.Role{},
			assertOutput: func(t testing.TB, options []string) {
				assert.Empty(t, options)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:     "API error",
			userID: "some-id",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "Failed to find the current roles for user with ID some-id: error.")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			userAPI := mock.NewMockUserAPI(ctrl)
			userAPI.EXPECT().
				Roles(gomock.Eq(test.userID), gomock.Any()).
				Return(&management.RoleList{ Roles: test.roles }, test.apiError)

			cli := &cli{
				api: &auth0.API{ User: userAPI },
			}

			options, err := userRolesToRemovePickerOptions(cli, test.userID)

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

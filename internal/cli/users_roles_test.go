package cli

import (
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestGetUserRoles(t *testing.T) {
	t.Run("gets user roles", func(t *testing.T) {
		inputs := &userRolesInput{
			ID:    "some-id",
			Roles: []string{},
		}
		userRolesFetcher := func(cli *cli, userID string) ([]string, error) {
			assert.Equal(t, userID, "some-id")
			return []string{"some-id-1", "some-id-2"}, nil
		}
		userRolesSelector := func(options []string) ([]string, error) {
			assert.Equal(t, options, []string{"some-id-1", "some-id-2"})
			return []string{"some-id-3 (Name: some-name-3)", "some-id-4 (Name: some-name-4)"}, nil
		}
		cli := &cli{}
		err := cli.getUserRoles(inputs, userRolesFetcher, userRolesSelector)

		assert.Equal(t, inputs.Roles, []string{"some-id-3", "some-id-4"})
		assert.Nil(t, err)
	})

	t.Run("returns error when user roles fetcher fails", func(t *testing.T) {
		inputs := &userRolesInput{Roles: []string{}}
		userRolesFetcher := func(cli *cli, userID string) ([]string, error) {
			return nil, errors.New("error")
		}
		cli := &cli{}
		err := cli.getUserRoles(inputs, userRolesFetcher, nil)

		assert.Error(t, err)
	})

	t.Run("returns error when user roles selector fails", func(t *testing.T) {
		inputs := &userRolesInput{Roles: []string{}}
		userRolesFetcher := func(cli *cli, userID string) ([]string, error) {
			return []string{}, nil
		}
		userRolesSelector := func(options []string) ([]string, error) {
			return nil, errors.New("error")
		}
		cli := &cli{}
		err := cli.getUserRoles(inputs, userRolesFetcher, userRolesSelector)

		assert.Error(t, err)
	})

	t.Run("returns error when no roles where selected", func(t *testing.T) {
		inputs := &userRolesInput{Roles: []string{}}
		userRolesFetcher := func(cli *cli, userID string) ([]string, error) {
			return []string{"some-id-1", "some-id-2"}, nil
		}
		userRolesSelector := func(options []string) ([]string, error) {
			return []string{}, nil
		}
		cli := &cli{}
		err := cli.getUserRoles(inputs, userRolesFetcher, userRolesSelector)

		assert.ErrorIs(t, err, errNoRolesSelected)
	})
}

func TestUserRolesToAddPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		userRoles    []*management.Role
		allRoles     []*management.Role
		userAPIError error
		roleAPIError error
		assertOutput func(t testing.TB, options []string)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			userRoles: []*management.Role{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			allRoles: []*management.Role{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
				{
					ID:   auth0.String("some-id-3"),
					Name: auth0.String("some-name-3"),
				},
				{
					ID:   auth0.String("some-id-4"),
					Name: auth0.String("some-name-4"),
				},
			},
			assertOutput: func(t testing.TB, options []string) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-id-3 (Name: some-name-3)", options[0])
				assert.Equal(t, "some-id-4 (Name: some-name-4)", options[1])
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:         "user API error",
			userID:       "some-id",
			userAPIError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "Failed to find the current roles for user with ID \"some-id\": error.")
			},
		},
		{
			name:         "role API error",
			roleAPIError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "Failed to list all roles: error.")
			},
		},
		{
			name:   "user already has all roles assigned",
			userID: "some-id",
			userRoles: []*management.Role{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			allRoles: []*management.Role{
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
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "The user with ID \"some-id\" has all roles assigned already.")
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
				Return(&management.RoleList{
					Roles: test.userRoles}, test.userAPIError)

			timesRolesAPIShouldBeCalled := 1

			if test.allRoles == nil && test.roleAPIError == nil {
				timesRolesAPIShouldBeCalled = 0
			}

			roleAPI := mock.NewMockRoleAPI(ctrl)
			roleAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.RoleList{Roles: test.allRoles}, test.roleAPIError).
				Times(timesRolesAPIShouldBeCalled)

			cli := &cli{
				api: &auth0.API{User: userAPI, Role: roleAPI},
			}

			options, err := userRolesToAddPickerOptions(cli, test.userID)

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestUserRolesToRemovePickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		userRoles    []*management.Role
		apiError     error
		assertOutput func(t testing.TB, options []string)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			userRoles: []*management.Role{
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
			name:      "no roles for user",
			userRoles: []*management.Role{},
			assertOutput: func(t testing.TB, options []string) {
				assert.Empty(t, options)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:     "API error",
			userID:   "some-id",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "Failed to find the current roles for user with ID \"some-id\": error.")
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
				Return(&management.RoleList{Roles: test.userRoles}, test.apiError)

			cli := &cli{
				api: &auth0.API{User: userAPI},
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

func TestContainsRole(t *testing.T) {
	t.Run("returns true when role is found", func(t *testing.T) {
		roles := []*management.Role{
			{
				ID: auth0.String("some-id-1"),
			},
			{
				ID: auth0.String("some-id-2"),
			},
		}

		result := containsRole(roles, "some-id-2")

		assert.True(t, result)
	})

	t.Run("returns false when role is not found", func(t *testing.T) {
		roles := []*management.Role{
			{
				ID: auth0.String("some-id-1"),
			},
			{
				ID: auth0.String("some-id-2"),
			},
		}

		result := containsRole(roles, "some-other-id")

		assert.False(t, result)
	})

	t.Run("returns false when there are no roles", func(t *testing.T) {
		roles := []*management.Role{}
		result := containsRole(roles, "some-id")

		assert.False(t, result)
	})
}

package cli

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestListRolesCmd(t *testing.T) {
	tests := []struct {
		name     string
		roleList *management.RoleList
	}{
		{
			name:     "nil role list (no results)",
			roleList: nil,
		},
		{
			name:     "empty role list",
			roleList: &management.RoleList{Roles: []*management.Role{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			roleAPI := mock.NewMockRoleAPI(ctrl)
			roleAPI.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(test.roleList, nil)

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				api: &auth0.API{Role: roleAPI},
			}

			cmd := listRolesCmd(cli)
			cmd.SetArgs([]string{})

			assert.NoError(t, cmd.Execute())
		})
	}
}

func TestRolesPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		roles        []*management.Role
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
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
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-id-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-id-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:  "no roles",
			roles: []*management.Role{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no roles to choose from. Create one by running: `auth0 roles create`")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			roleAPI := mock.NewMockRoleAPI(ctrl)
			roleAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.RoleList{
					Roles: test.roles}, test.apiError)

			cli := &cli{
				api: &auth0.API{Role: roleAPI},
			}

			options, err := cli.rolePickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

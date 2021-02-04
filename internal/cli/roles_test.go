package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/auth0.v5/management"
)

func TestRolesCmd(t *testing.T) {
	t.Run("List", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().List().MaxTimes(1).Return(&management.RoleList{List: management.List{}, Roles: []*management.Role{&management.Role{ID: auth0.String("testRoleID"), Name: auth0.String("testName"), Description: auth0.String("testDescription")}}}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesListCmd(cli)

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"ROLE ID", "NAME", "DESCRIPTION"},
			[][]string{
				{"testRoleID", "testName", "testDescription"},
			},
		)
	})

	t.Run("Get", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Read(gomock.AssignableToTypeOf("")).MaxTimes(1).Return(&management.Role{ID: auth0.String("testRoleID"), Name: auth0.String("testName"), Description: auth0.String("testDescription")}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesGetCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"ROLE ID", "NAME", "DESCRIPTION"},
			[][]string{
				{"testRoleID", "testName", "testDescription"},
			},
		)
	})

	t.Run("Delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Delete(gomock.AssignableToTypeOf("")).MaxTimes(1).Return(nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesDeleteCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Update(gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf(&management.Role{})).MaxTimes(1).Return(nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesUpdateCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID", "--name=testName", "--description=testDescription"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"ROLE ID", "NAME", "DESCRIPTION"},
			[][]string{
				{"", "testName", "testDescription"},
			},
		)
	})

	t.Run("Create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Create(gomock.AssignableToTypeOf(&management.Role{})).MaxTimes(1).Return(nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesCreateCmd(cli)
		cmd.SetArgs([]string{"--name=testName", "--description=testDescription"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"ROLE ID", "NAME", "DESCRIPTION"},
			[][]string{
				{"", "testName", "testDescription"},
			},
		)
	})

	t.Run("GetPermissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		permissions := []*management.Permission{
			&management.Permission{Name: auth0.String("testName"), ResourceServerIdentifier: auth0.String("testResourceServerIdentifier"), ResourceServerName: auth0.String("testResourceServerName"), Description: auth0.String("testDescription")},
		}
		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).MaxTimes(1).Return(&management.PermissionList{List: management.List{}, Permissions: permissions}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesGetPermissionsCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"ROLE ID", "PERMISSION NAME", "DESCRIPTION", "RESOURCE SERVICE IDENTIFIER", "RESOURCE SERVER NAME"},
			[][]string{
				{"testRoleID", "testName", "testDescription", "testResourceServerIdentifier", "testResourceServerName"},
			},
		)
	})

	t.Run("AssociatePermissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)

		m.EXPECT().AssociatePermissions(gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf([]*management.Permission{})).MaxTimes(1).DoAndReturn(func(r string, p []*management.Permission) error {
			assert.Equal(t, "testRoleID", r)
			assert.Equal(t, "testPermissionName1", p[0].GetName())
			assert.Equal(t, "testResourceServerIdentifier1", p[0].GetResourceServerIdentifier())
			assert.Equal(t, "testPermissionName2", p[1].GetName())
			assert.Equal(t, "testResourceServerIdentifier2", p[1].GetResourceServerIdentifier())
			return nil
		})

		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).MaxTimes(1).Return(&management.PermissionList{List: management.List{}, Permissions: nil}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesAssociatePermissionsCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID", "--permission-name=testPermissionName1", "--resource-server-identifier=testResourceServerIdentifier1", "--permission-name=testPermissionName2", "--resource-server-identifier=testResourceServerIdentifier2"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("RemovePermissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)

		m.EXPECT().RemovePermissions(gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf([]*management.Permission{})).MaxTimes(1).DoAndReturn(func(r string, p []*management.Permission) error {
			assert.Equal(t, "testRoleID", r)
			assert.Equal(t, "testPermissionName1", p[0].GetName())
			assert.Equal(t, "testResourceServerIdentifier1", p[0].GetResourceServerIdentifier())
			assert.Equal(t, "testPermissionName2", p[1].GetName())
			assert.Equal(t, "testResourceServerIdentifier2", p[1].GetResourceServerIdentifier())
			return nil
		})

		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).MaxTimes(1).Return(&management.PermissionList{List: management.List{}, Permissions: nil}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("table"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesRemovePermissionsCmd(cli)
		cmd.SetArgs([]string{"--role-id=testRoleID", "--permission-name=testPermissionName1", "--resource-server-identifier=testResourceServerIdentifier1", "--permission-name=testPermissionName2", "--resource-server-identifier=testResourceServerIdentifier2"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
}

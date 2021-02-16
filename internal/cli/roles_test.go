package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	t.Run("Get Many Roles", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Read(gomock.AssignableToTypeOf("")).Times(2).DoAndReturn(func(id string) (*management.Role, error) {
			return &management.Role{ID: auth0.String(id), Name: auth0.String("testName"), Description: auth0.String("testDescription")}, nil
		})
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesGetCmd(cli)
		cmd.SetArgs([]string{"testRoleID1", "testRoleID2"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		type result struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		results := make([]result, 2)
		if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, results, result{"testRoleID1", "testName", "testDescription"})
		assert.Contains(t, results, result{"testRoleID2", "testName", "testDescription"})
	})

	t.Run("Get a Single Role", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Read(gomock.AssignableToTypeOf("")).Times(1).DoAndReturn(func(id string) (*management.Role, error) {
			return &management.Role{ID: auth0.String(id), Name: auth0.String("testName"), Description: auth0.String("testDescription")}, nil
		})
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesGetCmd(cli)
		cmd.SetArgs([]string{"testRoleID1"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		type result struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		results := make([]result, 3)
		if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, results, result{"ROLE ID", "testRoleID1"})
		assert.Contains(t, results, result{"NAME", "testName"})
		assert.Contains(t, results, result{"DESCRIPTION", "testDescription"})
	})

	t.Run("Delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Delete(gomock.AssignableToTypeOf("")).Times(2).Return(nil)
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
		cmd.SetArgs([]string{"testRoleID1", "testRoleID2"})

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

	t.Run("Get a Single Role's Permissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		permissions := []*management.Permission{
			&management.Permission{Name: auth0.String("testName"), ResourceServerIdentifier: auth0.String("testResourceServerIdentifier")},
		}
		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).MaxTimes(1).Return(&management.PermissionList{List: management.List{}, Permissions: permissions}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesGetPermissionsCmd(cli)
		cmd.SetArgs([]string{"testRoleID1"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		assert.JSONEq(t, `[{"name": "ROLE ID", "value": "testRoleID1"},{"name": "PERMISSION NAME", "value": "testName"},{"name": "RESOURCE SERVER IDENTIFIER", "value": "testResourceServerIdentifier"}]`, stdout.String())
	})

	t.Run("AssociatePermissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)

		permissionName := "testPermissionName"
		resourceServerIdentifier := "testResourceServerIdentifier"
		permissions := []*management.Permission{
			&management.Permission{
				Name:                     auth0.String(permissionName),
				ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
			},
		}

		m.EXPECT().AssociatePermissions(gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf([]*management.Permission{})).Times(1).Return(nil)
		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).Times(1).Return(&management.PermissionList{List: management.List{}, Permissions: permissions}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesAssociatePermissionsCmd(cli)
		cmd.SetArgs([]string{"testRoleID", fmt.Sprintf("--permission-name=%s", permissionName), fmt.Sprintf("--resource-server-identifier=%s", resourceServerIdentifier)})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		assert.JSONEq(t, `[{"name": "ROLE ID", "value": "testRoleID"},{"name": "PERMISSION NAME", "value": "testPermissionName"},{"name": "RESOURCE SERVER IDENTIFIER", "value": "testResourceServerIdentifier"}]`, stdout.String())

	})

	t.Run("RemovePermissions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)

		permissionName := "testPermissionName"
		resourceServerIdentifier := "testResourceServerIdentifier"
		permissions := []*management.Permission{
			&management.Permission{
				Name:                     auth0.String(permissionName),
				ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
			},
		}

		m.EXPECT().RemovePermissions(gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf([]*management.Permission{})).Times(2).Return(nil)
		m.EXPECT().Permissions(gomock.AssignableToTypeOf(""), gomock.Any()).Times(2).Return(&management.PermissionList{List: management.List{}, Permissions: permissions}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{Role: m},
		}

		cmd := rolesRemovePermissionsCmd(cli)
		cmd.SetArgs([]string{"testRoleID1", "testRoleID2", "--permission-name=testPermissionName1", "--resource-server-identifier=testResourceServerIdentifier1"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		type result struct {
			ID                       string `json:"id"`
			PermissionName           string `json:"permission_name"`
			ResourceServerIdentifier string `json:"resource_server_identifier"`
		}
		results := make([]result, 2)
		if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, results, result{ID: "testRoleID1", PermissionName: permissionName, ResourceServerIdentifier: resourceServerIdentifier})
		assert.Contains(t, results, result{ID: "testRoleID2", PermissionName: permissionName, ResourceServerIdentifier: resourceServerIdentifier})
	})
}

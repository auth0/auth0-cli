package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"gopkg.in/auth0.v5/management"
)

func TestRolesCmd(t *testing.T) {
	t.Run("List", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().List().MaxTimes(1).Return(&management.RoleList{List: management.List{}, Roles: []*management.Role{&management.Role{ID: auth0.String("testID"), Name: auth0.String("testName"), Description: auth0.String("testDescription")}}}, nil)
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
			[]string{"NAME", "ROLE ID", "DESCRIPTION"},
			[][]string{
				{"testName", "testID", "testDescription"},
			},
		)
	})

	t.Run("Get", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockRoleAPI(ctrl)
		m.EXPECT().Read(gomock.AssignableToTypeOf("")).MaxTimes(1).Return(&management.Role{ID: auth0.String("testID"), Name: auth0.String("testName"), Description: auth0.String("testDescription")}, nil)
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
		cmd.SetArgs([]string{"--role-id=testID"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"NAME", "ROLE ID", "DESCRIPTION"},
			[][]string{
				{"testName", "testID", "testDescription"},
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
		cmd.SetArgs([]string{"--role-id=testID"})

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
		cmd.SetArgs([]string{"--role-id=testID", "--name=testName", "--description=testDescription"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		expectTable(t, stdout.String(),
			[]string{"NAME", "ROLE ID", "DESCRIPTION"},
			[][]string{
				{"testName", "", "testDescription"},
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
			[]string{"NAME", "ROLE ID", "DESCRIPTION"},
			[][]string{
				{"testName", "", "testDescription"},
			},
		)
	})
}

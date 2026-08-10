package cli

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func actionModulePage(results ...*managementv3.ActionModuleListItem) *auth0.ActionModulePage {
	return &auth0.ActionModulePage{
		Results: results,
		NextPageFunc: func(context.Context) (*auth0.ActionModulePage, error) {
			return nil, core.ErrNoPages
		},
	}
}

func TestListActionModulesCmd(t *testing.T) {
	t.Run("lists the action modules", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(actionModulePage(
				&managementv3.ActionModuleListItem{ID: auth0.String("am_1"), Name: auth0.String("one")},
				&managementv3.ActionModuleListItem{ID: auth0.String("am_2"), Name: auth0.String("two")},
			), nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := listActionModulesCmd(cli)
		cmd.SetArgs([]string{})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("renders an empty state when there are no modules", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(actionModulePage(), nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := listActionModulesCmd(cli)
		cmd.SetArgs([]string{})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("rejects an out-of-range number flag", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: mock.NewMockActionModuleAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := listActionModulesCmd(cli)
		cmd.SetArgs([]string{"--number", "0"})

		assert.EqualError(t, cmd.Execute(), "number flag invalid, please pass a number between 1 and 1000")
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := listActionModulesCmd(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), "failed to list action modules: boom")
	})
}

func TestShowActionModuleCmd(t *testing.T) {
	t.Run("shows the module supplied positionally", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Get(gomock.Any(), "am_1").
			Return(&managementv3.GetActionModuleResponseContent{
				ID:   auth0.String("am_1"),
				Name: auth0.String("one"),
			}, nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := showActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Get(gomock.Any(), "am_1").
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := showActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.EqualError(t, cmd.Execute(), `failed to read action module with ID "am_1": boom`)
	})
}

func TestCreateActionModuleCmd(t *testing.T) {
	t.Run("creates a module with all flags mapped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateActionModuleRequestContent, _ ...option.RequestOption) (*managementv3.CreateActionModuleResponseContent, error) {
				assert.Equal(t, "mymodule", req.Name)
				assert.Equal(t, "module.exports = () => {}", req.Code)
				assert.Equal(t, auth0.Bool(true), req.Publish)
				assert.Equal(t, auth0.String("v1"), req.APIVersion)

				assert.Len(t, req.Dependencies, 1)
				assert.Equal(t, "lodash", req.Dependencies[0].Name)
				assert.Equal(t, "4.0.0", req.Dependencies[0].Version)

				assert.Len(t, req.Secrets, 1)
				assert.Equal(t, "API_KEY", req.Secrets[0].Name)
				assert.Equal(t, "secret-value", req.Secrets[0].Value)

				return &managementv3.CreateActionModuleResponseContent{
					ID:   auth0.String("am_1"),
					Name: auth0.String("mymodule"),
				}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := createActionModuleCmd(cli)
		cmd.SetArgs([]string{
			"--name", "mymodule",
			"--code", "module.exports = () => {}",
			"--dependency", "lodash=4.0.0",
			"--secret", "API_KEY=secret-value",
			"--api-version", "v1",
			"--publish",
		})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("omits publish and api-version when not set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateActionModuleRequestContent, _ ...option.RequestOption) (*managementv3.CreateActionModuleResponseContent, error) {
				assert.Nil(t, req.Publish)
				assert.Nil(t, req.APIVersion)
				assert.Nil(t, req.Dependencies)
				assert.Nil(t, req.Secrets)

				return &managementv3.CreateActionModuleResponseContent{ID: auth0.String("am_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := createActionModuleCmd(cli)
		cmd.SetArgs([]string{"--name", "mymodule", "--code", "code"})

		assert.NoError(t, cmd.Execute())
	})
}

func TestUpdateActionModuleCmd(t *testing.T) {
	t.Run("only sends the fields that were set and never a name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Update(gomock.Any(), "am_1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateActionModuleRequestContent, _ ...option.RequestOption) (*managementv3.UpdateActionModuleResponseContent, error) {
				assert.Equal(t, auth0.String("new code"), req.Code)
				assert.Nil(t, req.Dependencies)
				assert.Nil(t, req.Secrets)

				return &managementv3.UpdateActionModuleResponseContent{ID: auth0.String("am_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--code", "new code"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("leaves code nil when only dependencies change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Update(gomock.Any(), "am_1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateActionModuleRequestContent, _ ...option.RequestOption) (*managementv3.UpdateActionModuleResponseContent, error) {
				assert.Nil(t, req.Code)
				assert.Len(t, req.Dependencies, 1)

				return &managementv3.UpdateActionModuleResponseContent{ID: auth0.String("am_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--dependency", "lodash=4.0.0"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("updates then publishes when --publish leaves unpublished changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)

		gomock.InOrder(
			api.EXPECT().
				Update(gomock.Any(), "am_1", gomock.Any()).
				Return(&managementv3.UpdateActionModuleResponseContent{
					ID:                  auth0.String("am_1"),
					AllChangesPublished: auth0.Bool(false),
				}, nil),
			versions.EXPECT().
				Create(gomock.Any(), "am_1").
				Return(&managementv3.CreateActionModuleVersionResponseContent{VersionNumber: auth0.Int(2)}, nil),
			api.EXPECT().
				Get(gomock.Any(), "am_1").
				Return(&managementv3.GetActionModuleResponseContent{ID: auth0.String("am_1")}, nil),
		)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api, ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--code", "new code", "--publish"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("skips publish when the draft is already fully published", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)

		gomock.InOrder(
			api.EXPECT().
				Update(gomock.Any(), "am_1", gomock.Any()).
				Return(&managementv3.UpdateActionModuleResponseContent{
					ID:                  auth0.String("am_1"),
					AllChangesPublished: auth0.Bool(true),
				}, nil),
			api.EXPECT().
				Get(gomock.Any(), "am_1").
				Return(&managementv3.GetActionModuleResponseContent{ID: auth0.String("am_1")}, nil),
		)
		// Versions.Create must never be called when everything is published.

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api, ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--code", "new code", "--publish"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("publishes an existing draft without field changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)

		gomock.InOrder(
			api.EXPECT().
				Get(gomock.Any(), "am_1").
				Return(&managementv3.GetActionModuleResponseContent{
					ID:                  auth0.String("am_1"),
					AllChangesPublished: auth0.Bool(false),
				}, nil),
			versions.EXPECT().
				Create(gomock.Any(), "am_1").
				Return(&managementv3.CreateActionModuleVersionResponseContent{VersionNumber: auth0.Int(2)}, nil),
			api.EXPECT().
				Get(gomock.Any(), "am_1").
				Return(&managementv3.GetActionModuleResponseContent{ID: auth0.String("am_1")}, nil),
		)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api, ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--publish"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("does not call the API when there are no changes and no publish", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Get(gomock.Any(), "am_1").
			Return(&managementv3.GetActionModuleResponseContent{ID: auth0.String("am_1")}, nil)
		// Update must never be called for a no-op.

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := updateActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.NoError(t, cmd.Execute())
	})
}

func TestDeleteActionModuleCmd(t *testing.T) {
	t.Run("deletes the module supplied positionally", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Delete(gomock.Any(), "am_1").
			Return(nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := deleteActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--force"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("deletes multiple modules supplied positionally", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Delete(gomock.Any(), "am_1").
			Return(nil)
		api.EXPECT().
			Delete(gomock.Any(), "am_2").
			Return(nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := deleteActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "am_2", "--force"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Delete(gomock.Any(), "am_1").
			Return(errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := deleteActionModuleCmd(cli)
		cmd.SetArgs([]string{"am_1", "--force"})

		assert.EqualError(t, cmd.Execute(), `failed to delete action module with ID "am_1": boom`)
	})
}

func TestDeletableActionModulePickerOptions(t *testing.T) {
	t.Run("hides modules in use by deployed action versions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(actionModulePage(
				&managementv3.ActionModuleListItem{ID: auth0.String("am_free"), Name: auth0.String("free"), ActionsUsingModuleTotal: auth0.Int(0)},
				&managementv3.ActionModuleListItem{ID: auth0.String("am_used"), Name: auth0.String("used"), ActionsUsingModuleTotal: auth0.Int(1)},
			), nil)

		cli := &cli{apiv3: &auth0.APIV3{ActionModule: api}}

		opts, err := cli.deletableActionModulePickerOptions(context.Background())
		assert.NoError(t, err)
		assert.Len(t, opts, 1)
		assert.Equal(t, "am_free", opts[0].value)
	})

	t.Run("errors when every module is in use", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(actionModulePage(
				&managementv3.ActionModuleListItem{ID: auth0.String("am_used"), Name: auth0.String("used"), ActionsUsingModuleTotal: auth0.Int(1)},
			), nil)

		cli := &cli{apiv3: &auth0.APIV3{ActionModule: api}}

		_, err := cli.deletableActionModulePickerOptions(context.Background())
		assert.ErrorContains(t, err, "in use by deployed action versions")
	})
}

func TestInputDependenciesToActionModuleDependencies(t *testing.T) {
	assert.Nil(t, inputDependenciesToActionModuleDependencies(nil))

	result := inputDependenciesToActionModuleDependencies(map[string]string{"lodash": "4.0.0"})
	assert.Len(t, result, 1)
	assert.Equal(t, "lodash", result[0].Name)
	assert.Equal(t, "4.0.0", result[0].Version)
}

func TestInputSecretsToActionModuleSecrets(t *testing.T) {
	assert.Nil(t, inputSecretsToActionModuleSecrets(nil))

	result := inputSecretsToActionModuleSecrets(map[string]string{"API_KEY": "value"})
	assert.Len(t, result, 1)
	assert.Equal(t, "API_KEY", result[0].Name)
	assert.Equal(t, "value", result[0].Value)
}

func TestPluralize(t *testing.T) {
	assert.Equal(t, "0 secrets", pluralize(0, "secret", "secrets"))
	assert.Equal(t, "1 secret", pluralize(1, "secret", "secrets"))
	assert.Equal(t, "2 secrets", pluralize(2, "secret", "secrets"))
	assert.Equal(t, "1 dependency", pluralize(1, "dependency", "dependencies"))
	assert.Equal(t, "3 dependencies", pluralize(3, "dependency", "dependencies"))
}

func TestSummarizeActionModuleDependencies(t *testing.T) {
	t.Run("single dependency", func(t *testing.T) {
		assert.Equal(t, "1 dependency: lodash@4.17.21",
			summarizeActionModuleDependencies(map[string]string{"lodash": "4.17.21"}))
	})

	t.Run("multiple dependencies are sorted for a stable recap", func(t *testing.T) {
		assert.Equal(t, "2 dependencies: auth0@1.4.0, lodash@4.17.21",
			summarizeActionModuleDependencies(map[string]string{
				"lodash": "4.17.21",
				"auth0":  "1.4.0",
			}))
	})
}

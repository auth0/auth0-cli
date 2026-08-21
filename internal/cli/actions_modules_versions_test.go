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

func actionModuleVersionPage(results ...*managementv3.ActionModuleVersion) *auth0.ActionModuleVersionPage {
	return &auth0.ActionModuleVersionPage{
		Results: results,
		NextPageFunc: func(context.Context) (*auth0.ActionModuleVersionPage, error) {
			return nil, core.ErrNoPages
		},
	}
}

func TestListActionModuleVersionsCmd(t *testing.T) {
	t.Run("lists the versions of the module supplied positionally", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			List(gomock.Any(), "am_1", gomock.Any()).
			Return(actionModuleVersionPage(
				&managementv3.ActionModuleVersion{ID: auth0.String("ver_1"), VersionNumber: auth0.Int(1)},
				&managementv3.ActionModuleVersion{ID: auth0.String("ver_2"), VersionNumber: auth0.Int(2)},
			), nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := listActionModuleVersionsCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("rejects an out-of-range number flag", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModuleVersion: mock.NewMockActionModuleVersionAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := listActionModuleVersionsCmd(cli)
		cmd.SetArgs([]string{"am_1", "--number", "0"})

		assert.EqualError(t, cmd.Execute(), "number flag invalid, please pass a number between 1 and 1000")
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			List(gomock.Any(), "am_1", gomock.Any()).
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := listActionModuleVersionsCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.EqualError(t, cmd.Execute(), `failed to list versions for action module with ID "am_1": boom`)
	})
}

func TestShowActionModuleVersionCmd(t *testing.T) {
	t.Run("shows the version supplied positionally", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			Get(gomock.Any(), "am_1", "ver_1").
			Return(&managementv3.GetActionModuleVersionResponseContent{
				ID:            auth0.String("ver_1"),
				VersionNumber: auth0.Int(1),
			}, nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := showActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1", "ver_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			Get(gomock.Any(), "am_1", "ver_1").
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := showActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1", "ver_1"})

		assert.EqualError(t, cmd.Execute(), `failed to read version "ver_1" of action module with ID "am_1": boom`)
	})
}

func TestRollbackActionModuleVersionCmd(t *testing.T) {
	t.Run("rolls back then re-reads the module", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)

		gomock.InOrder(
			api.EXPECT().
				Rollback(gomock.Any(), "am_1", gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, req *managementv3.RollbackActionModuleRequestParameters, _ ...option.RequestOption) (*managementv3.RollbackActionModuleResponseContent, error) {
					assert.Equal(t, "ver_1", req.ModuleVersionID)
					return &managementv3.RollbackActionModuleResponseContent{ID: auth0.String("am_1")}, nil
				}),
			api.EXPECT().
				Get(gomock.Any(), "am_1").
				Return(&managementv3.GetActionModuleResponseContent{ID: auth0.String("am_1")}, nil),
		)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := rollbackActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1", "ver_1", "--force"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		api.EXPECT().
			Rollback(gomock.Any(), "am_1", gomock.Any()).
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api},
			renderer: testRenderer(),
		}

		cmd := rollbackActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1", "ver_1", "--force"})

		assert.EqualError(t, cmd.Execute(), `failed to roll back action module with ID "am_1" to version "ver_1": boom`)
	})
}

func TestPublishActionModuleVersionCmd(t *testing.T) {
	t.Run("publishes when the draft has unpublished changes", func(t *testing.T) {
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

		cmd := publishActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("skips publish when the draft is already fully published", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := mock.NewMockActionModuleAPIV3(ctrl)
		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		// Versions.Create must never be called when everything is published.

		api.EXPECT().
			Get(gomock.Any(), "am_1").
			Return(&managementv3.GetActionModuleResponseContent{
				ID:                  auth0.String("am_1"),
				AllChangesPublished: auth0.Bool(true),
			}, nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{ActionModule: api, ActionModuleVersion: versions},
			renderer: testRenderer(),
		}

		cmd := publishActionModuleVersionCmd(cli)
		cmd.SetArgs([]string{"am_1"})

		assert.NoError(t, cmd.Execute())
	})
}

func TestActionModuleVersionPickerOptions(t *testing.T) {
	t.Run("labels versions by number and id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			List(gomock.Any(), "am_1", gomock.Any()).
			Return(actionModuleVersionPage(
				&managementv3.ActionModuleVersion{ID: auth0.String("ver_2"), VersionNumber: auth0.Int(2)},
			), nil)

		cli := &cli{apiv3: &auth0.APIV3{ActionModuleVersion: versions}}

		opts, err := cli.actionModuleVersionPickerOptions("am_1")(context.Background())
		assert.NoError(t, err)
		assert.Len(t, opts, 1)
		assert.Equal(t, "ver_2", opts[0].value)
	})

	t.Run("errors when the module has no versions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		versions := mock.NewMockActionModuleVersionAPIV3(ctrl)
		versions.EXPECT().
			List(gomock.Any(), "am_1", gomock.Any()).
			Return(actionModuleVersionPage(), nil)

		cli := &cli{apiv3: &auth0.APIV3{ActionModuleVersion: versions}}

		_, err := cli.actionModuleVersionPickerOptions("am_1")(context.Background())
		assert.ErrorContains(t, err, "no published versions")
	})
}

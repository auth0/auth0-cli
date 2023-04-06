package cli

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestActionsDeployCmd(t *testing.T) {
	t.Run("it successfully deploys an action", func(t *testing.T) {
		actionID := "1221c74c-cfd6-40db-af13-7bc9bb1c38db"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			Deploy(actionID).
			Return(nil, nil)

		actionAPI.EXPECT().
			Read(actionID).
			Return(&management.Action{
				ID:   auth0.String(actionID),
				Name: auth0.String("actions-deploy"),
				SupportedTriggers: []management.ActionTrigger{
					{
						ID: auth0.String("post-login"),
					},
				},
				Code: auth0.String("function () {}"),
				DeployedVersion: &management.ActionVersion{
					Deployed: true,
				},
				Status: auth0.String("built"),
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			api: &auth0.API{Action: actionAPI},
		}

		cmd := deployActionCmd(cli)
		cmd.SetArgs([]string{actionID})
		err := cmd.Execute()

		assert.NoError(t, err)
		expectTable(t, stdout.String(),
			[]string{},
			[][]string{
				{"ID             1221c74c-cfd6-40db-af13-7bc9bb1c38db"},
				{"NAME           actions-deploy"},
				{"TYPE           post-login"},
				{"STATUS         built"},
				{"DEPLOYED       ✓"},
				{"LAST DEPLOYED"},
				{"LAST UPDATED   Jan 01 0001"},
				{"CREATED        Jan 01 0001"},
				{"CODE           function () {}"},
			},
		)
	})

	t.Run("it returns an error if it fails to deploy the action", func(t *testing.T) {
		actionID := "1221c74c-cfd6-40db-af13-7bc9bb1c38db"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			Deploy(actionID).
			Return(nil, fmt.Errorf("400 Bad Request"))

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			api: &auth0.API{Action: actionAPI},
		}

		cmd := deployActionCmd(cli)
		cmd.SetArgs([]string{actionID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to deploy action with ID "1221c74c-cfd6-40db-af13-7bc9bb1c38db": 400 Bad Request`)
	})

	t.Run("it returns an error if it fails to read the action", func(t *testing.T) {
		actionID := "1221c74c-cfd6-40db-af13-7bc9bb1c38db"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			Deploy(actionID).
			Return(nil, nil)

		actionAPI.EXPECT().
			Read(actionID).
			Return(nil, fmt.Errorf("400 Bad Request"))

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			api: &auth0.API{Action: actionAPI},
		}

		cmd := deployActionCmd(cli)
		cmd.SetArgs([]string{actionID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to get deployed action with ID "1221c74c-cfd6-40db-af13-7bc9bb1c38db": 400 Bad Request`)
	})
}

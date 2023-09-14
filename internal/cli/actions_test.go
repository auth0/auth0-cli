package cli

import (
	"bytes"
	"context"
	"errors"
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
			Deploy(context.Background(), actionID).
			Return(nil, nil)

		actionAPI.EXPECT().
			Read(context.Background(), actionID).
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
			Deploy(context.Background(), actionID).
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
			Deploy(context.Background(), actionID).
			Return(nil, nil)

		actionAPI.EXPECT().
			Read(context.Background(), actionID).
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

func TestActionsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		actions      []*management.Action
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			actions: []*management.Action{
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
			name:    "no actions",
			actions: []*management.Action{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "There are currently no actions.")
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

			actionAPI := mock.NewMockActionAPI(ctrl)
			actionAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ActionList{
					Actions: test.actions}, test.apiError)

			cli := &cli{
				api: &auth0.API{Action: actionAPI},
			}

			options, err := cli.actionPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
func TestActionsInputSecretsToActionSecrets(t *testing.T) {
	t.Run("it should map input secrets to action payload", func(t *testing.T) {
		input := map[string]string{
			"secret1": "value1",
			"secret2": "value2",
			"secret3": "value3",
		}
		res := inputSecretsToActionSecrets(input)
		expected := []management.ActionSecret{
			{
				Name:  auth0.String("secret1"),
				Value: auth0.String("value1"),
			},
			{
				Name:  auth0.String("secret2"),
				Value: auth0.String("value2"),
			},
			{
				Name:  auth0.String("secret3"),
				Value: auth0.String("value3"),
			},
		}
		assert.Len(t, *res, 3)
		assert.Equal(t, *res, expected)
	})

	t.Run("it should handle empty input secrets", func(t *testing.T) {
		emptyInput := map[string]string{}
		res := inputSecretsToActionSecrets(emptyInput)
		expected := []management.ActionSecret{}
		assert.Len(t, *res, 0)
		assert.Equal(t, res, &expected)
	})
}

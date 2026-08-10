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

		assert.EqualError(t, err, `failed to read deployed action with ID "1221c74c-cfd6-40db-af13-7bc9bb1c38db": 400 Bad Request`)
	})
}

func TestActionsUpdateCmd(t *testing.T) {
	t.Run("it carries the existing name forward on a module-only update", func(t *testing.T) {
		actionID := "1221c74c-cfd6-40db-af13-7bc9bb1c38db"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			Read(context.Background(), actionID).
			Return(&management.Action{
				ID:   auth0.String(actionID),
				Name: auth0.String("existing-name"),
				SupportedTriggers: []management.ActionTrigger{
					{ID: auth0.String("post-login")},
				},
				Code: auth0.String("function () {}"),
			}, nil)

		var captured *management.Action
		actionAPI.EXPECT().
			Update(context.Background(), actionID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, a *management.Action, _ ...management.RequestOption) error {
				captured = a
				return nil
			})

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			api: &auth0.API{Action: actionAPI},
		}

		cmd := updateActionCmd(cli)
		cmd.SetArgs([]string{
			actionID,
			"--module", "module_id=mod_123,module_version_id=ver_456",
		})
		err := cmd.Execute()

		assert.NoError(t, err)
		assert.Equal(t, "existing-name", captured.GetName())
		assert.Equal(t, []management.ActionModules{
			{ModuleID: auth0.String("mod_123"), ModuleVersionID: auth0.String("ver_456")},
		}, *captured.Modules)
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
				assert.ErrorContains(t, err, "there are currently no actions to choose from. Create one by running: `auth0 actions create`")
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

func TestUndeployedActionsPickerOptions(t *testing.T) {
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
				assert.ErrorContains(t, err, "there are currently no actions to deploy")
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
				List(gomock.Any(), gomock.Any()).
				Return(&management.ActionList{
					Actions: test.actions}, test.apiError)

			cli := &cli{
				api: &auth0.API{Action: actionAPI},
			}

			options, err := cli.unDeployedActionPickerOptions(context.Background())

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

		assert.Len(t, *res, 3)
		assert.Contains(t, *res, management.ActionSecret{
			Name:  auth0.String("secret1"),
			Value: auth0.String("value1"),
		})
		assert.Contains(t, *res, management.ActionSecret{
			Name:  auth0.String("secret2"),
			Value: auth0.String("value2"),
		})
		assert.Contains(t, *res, management.ActionSecret{
			Name:  auth0.String("secret3"),
			Value: auth0.String("value3"),
		})
	})

	t.Run("it should handle empty input secrets", func(t *testing.T) {
		emptyInput := map[string]string{}
		res := inputSecretsToActionSecrets(emptyInput)
		expected := []management.ActionSecret{}
		assert.Len(t, *res, 0)
		assert.Equal(t, res, &expected)
	})
}
func TestActionsInputDependenciesToActionDependencies(t *testing.T) {
	t.Run("it should map input dependencies to action payload", func(t *testing.T) {
		input := map[string]string{
			"fs-extra": "11.1.1",
			"lodash":   "4.0.0",
			"uuid":     "9.0.0",
		}
		res := inputDependenciesToActionDependencies(input)

		assert.Len(t, *res, 3)
		assert.Contains(t, *res, management.ActionDependency{
			Name:    auth0.String("fs-extra"),
			Version: auth0.String("11.1.1"),
		})
		assert.Contains(t, *res, management.ActionDependency{
			Name:    auth0.String("lodash"),
			Version: auth0.String("4.0.0"),
		})
		assert.Contains(t, *res, management.ActionDependency{
			Name:    auth0.String("uuid"),
			Version: auth0.String("9.0.0"),
		})
	})

	t.Run("it should handle empty input dependencies", func(t *testing.T) {
		emptyInput := map[string]string{}
		res := inputDependenciesToActionDependencies(emptyInput)
		expected := []management.ActionDependency{}
		assert.Len(t, *res, 0)
		assert.Equal(t, expected, *res)
	})
}

func TestActionsInputModulesToActionModules(t *testing.T) {
	t.Run("it maps a single module with id and version id", func(t *testing.T) {
		res, err := inputModulesToActionModules([]string{"module_id=mod_123,module_version_id=ver_456"})

		assert.NoError(t, err)
		assert.Equal(t, []management.ActionModules{
			{ModuleID: auth0.String("mod_123"), ModuleVersionID: auth0.String("ver_456")},
		}, *res)
	})

	t.Run("it maps multiple modules", func(t *testing.T) {
		res, err := inputModulesToActionModules([]string{
			"module_id=mod_123,module_version_id=ver_456",
			"module_id=mod_789,module_version_id=ver_abc",
		})

		assert.NoError(t, err)
		assert.Equal(t, []management.ActionModules{
			{ModuleID: auth0.String("mod_123"), ModuleVersionID: auth0.String("ver_456")},
			{ModuleID: auth0.String("mod_789"), ModuleVersionID: auth0.String("ver_abc")},
		}, *res)
	})

	t.Run("it handles empty input modules", func(t *testing.T) {
		res, err := inputModulesToActionModules([]string{})
		expected := []management.ActionModules{}
		assert.NoError(t, err)
		assert.Equal(t, &expected, res)
	})

	t.Run("it errors on a malformed pair", func(t *testing.T) {
		_, err := inputModulesToActionModules([]string{"mod_123"})
		assert.ErrorContains(t, err, "expected comma-separated key=value pairs")
	})

	t.Run("it errors when module_id is missing", func(t *testing.T) {
		_, err := inputModulesToActionModules([]string{"module_version_id=ver_456"})
		assert.ErrorContains(t, err, "module_id is required")
	})

	t.Run("it errors when module_version_id is missing", func(t *testing.T) {
		_, err := inputModulesToActionModules([]string{"module_id=mod_123"})
		assert.ErrorContains(t, err, "module_version_id is required")
	})

	t.Run("it errors on an unknown key", func(t *testing.T) {
		_, err := inputModulesToActionModules([]string{"module_id=mod_123,foo=bar"})
		assert.ErrorContains(t, err, "unknown key")
	})
}

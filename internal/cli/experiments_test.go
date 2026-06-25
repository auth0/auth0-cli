package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	management "github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestExperimentsListCmd(t *testing.T) {
	tests := []struct {
		name          string
		experiments   []*management.ExperimentListItem
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully lists experiments",
			experiments: []*management.ExperimentListItem{
				{
					ID:             "exp_001",
					Name:           "button-color",
					Status:         management.ExperimentStatusEnumDraft,
					FeatureFlagID:  "ff_001",
					IsValid:        false,
				},
				{
					ID:             "exp_002",
					Name:           "checkout-flow",
					Status:         management.ExperimentStatusEnumActive,
					FeatureFlagID:  "ff_002",
					IsValid:        true,
				},
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "button-color")
				assert.Contains(t, out, "exp_001")
				assert.Contains(t, out, "checkout-flow")
				assert.Contains(t, out, "exp_002")
			},
		},
		{
			name:        "it displays an empty state when there are no experiments",
			experiments: []*management.ExperimentListItem{},
			assertOutput: func(t testing.TB, out string) {
				assert.Empty(t, out)
			},
		},
		{
			name:          "it returns an error if the API call fails",
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: "failed to list experiments: 500 Internal Server Error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			experimentAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(
					&managementcore.Page[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent]{
						Results:      test.experiments,
						NextPageFunc: noNextPage[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent](),
					},
					test.apiError,
				)

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv2: &auth0.APIV2{Experiments: experimentAPI},
			}

			cmd := listExperimentsCmd(cli)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				test.assertOutput(t, stdout.String())
			}
		})
	}
}

func TestExperimentsShowCmd(t *testing.T) {
	const expID = "exp_abc123"

	t.Run("it successfully shows an experiment", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Get(gomock.Any(), expID).
			Return(&management.GetExperimentResponseContent{
				ID:                 expID,
				Name:               "button-color",
				Status:             management.ExperimentStatusEnumDraft,
				FeatureFlagID:      "ff_001",
				AuthenticationFlow: "login",
				AllocationStrategy: management.AllocationStrategyEnumPercentage,
				IsValid:            false,
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := showExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "button-color")
		assert.Contains(t, stdout.String(), expID)
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Get(gomock.Any(), expID).
			Return(nil, errors.New("404 Not Found"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := showExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to get experiment "exp_abc123": 404 Not Found`)
	})
}

func TestExperimentsCreateCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.CreateExperimentResponseContent
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully creates an experiment",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "login",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":0.5,"is_control":true},{"variation_id":"vid_002","weight":0.5,"is_control":false}]`,
			},
			apiResponse: &management.CreateExperimentResponseContent{
				ID:                 "exp_new",
				Name:               "button-color",
				Status:             management.ExperimentStatusEnumDraft,
				FeatureFlagID:      "ff_001",
				AuthenticationFlow: "login",
				AllocationStrategy: management.AllocationStrategyEnumPercentage,
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "button-color")
				assert.Contains(t, out, "exp_new")
			},
		},
		{
			name: "it returns an error when --allocations is empty",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "login",
				"--allocation-strategy", "percentage",
				"--allocations", "",
			},
			expectedError: "--allocations is required",
		},
		{
			name: "it returns an error when --allocations is invalid JSON",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "login",
				"--allocation-strategy", "percentage",
				"--allocations", "not-json",
			},
			expectedError: "invalid JSON for --allocations",
		},
		{
			name: "it returns an error if the API call fails",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "login",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":1.0,"is_control":true}]`,
			},
			apiError:      errors.New("400 Bad Request"),
			expectedError: "failed to create experiment: 400 Bad Request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			if test.apiResponse != nil || test.apiError != nil {
				experimentAPI.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv2: &auth0.APIV2{Experiments: experimentAPI},
			}

			cmd := createExperimentCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				test.assertOutput(t, stdout.String())
			}
		})
	}
}

func TestExperimentsUpdateCmd(t *testing.T) {
	const expID = "exp_abc123"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.UpdateExperimentResponseContent
		apiError      error
		expectedError string
	}{
		{
			name:        "it successfully updates the name",
			args:        []string{expID, "--name", "new-name"},
			apiResponse: &management.UpdateExperimentResponseContent{ID: expID, Name: "new-name"},
		},
		{
			name:          "it returns an error when no flags are provided",
			args:          []string{expID},
			expectedError: "nothing to update",
		},
		{
			name:          "it returns an error when --allocations is invalid JSON",
			args:          []string{expID, "--allocations", "not-json"},
			expectedError: "invalid JSON for --allocations",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{expID, "--name", "new-name"},
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: `failed to update experiment "exp_abc123": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			if test.apiResponse != nil || test.apiError != nil {
				experimentAPI.EXPECT().
					Update(gomock.Any(), expID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv2: &auth0.APIV2{Experiments: experimentAPI},
			}

			cmd := updateExperimentCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExperimentsDeleteCmd(t *testing.T) {
	const expID = "exp_abc123"

	t.Run("it successfully deletes an experiment with --force", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Delete(gomock.Any(), expID).
			Return(nil)

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
			force: true,
		}

		cmd := deleteExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.NoError(t, err)
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Delete(gomock.Any(), expID).
			Return(errors.New("500 Internal Server Error"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
			force: true,
		}

		cmd := deleteExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to delete experiment "exp_abc123": 500 Internal Server Error`)
	})
}

func TestExperimentsValidateCmd(t *testing.T) {
	const expID = "exp_abc123"

	t.Run("it shows valid when the experiment passes validation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Validate(gomock.Any(), expID).
			Return(&management.ValidateExperimentResponseContent{
				IsValid: true,
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := validateExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "✓")
	})

	t.Run("it shows invalid and errors when the experiment fails validation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Validate(gomock.Any(), expID).
			Return(&management.ValidateExperimentResponseContent{
				IsValid: false,
				Errors: []*management.ExperimentValidationError{
					{
						Code:    "missing_control_variation",
						Message: "Exactly one variation must be marked as control.",
					},
				},
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := validateExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "✗")
		assert.Contains(t, stdout.String(), "missing_control_variation")
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			Validate(gomock.Any(), expID).
			Return(nil, errors.New("500 Internal Server Error"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := validateExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to validate experiment "exp_abc123": 500 Internal Server Error`)
	})
}

func TestExperimentsStartCmd(t *testing.T) {
	const expID = "exp_abc123"

	t.Run("it successfully starts an experiment", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			UpdateStatus(gomock.Any(), expID, &management.UpdateExperimentStatusRequestContent{
				Status: management.ExperimentTransitionStatusEnum("active"),
			}).
			Return(&management.UpdateExperimentStatusResponseContent{
				ID:     expID,
				Name:   "button-color",
				Status: management.ExperimentStatusEnumActive,
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := startExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.NoError(t, err)
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		experimentAPI := mock.NewMockExperimentsAPI(ctrl)
		experimentAPI.EXPECT().
			UpdateStatus(gomock.Any(), expID, gomock.Any()).
			Return(nil, errors.New("400 Bad Request"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv2: &auth0.APIV2{Experiments: experimentAPI},
		}

		cmd := startExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to set experiment "exp_abc123" to active: 400 Bad Request`)
	})
}

func TestExperimentPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		experiments  []*management.ExperimentListItem
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "it returns picker options for each experiment",
			experiments: []*management.ExperimentListItem{
				{ID: "exp_001", Name: "button-color", Status: management.ExperimentStatusEnumDraft},
				{ID: "exp_002", Name: "checkout-flow", Status: management.ExperimentStatusEnumActive},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "exp_001", options[0].value)
				assert.Equal(t, "exp_002", options[1].value)
				assert.Contains(t, options[0].label, "button-color")
				assert.Contains(t, options[0].label, "exp_001")
				assert.Contains(t, options[1].label, "checkout-flow")
				assert.Contains(t, options[1].label, "exp_002")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "it returns an error when there are no experiments",
			experiments: []*management.ExperimentListItem{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "no experiments available")
			},
		},
		{
			name:     "it returns an error if the API call fails",
			apiError: errors.New("500 Internal Server Error"),
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

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			experimentAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(
					&managementcore.Page[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent]{
						Results:      test.experiments,
						NextPageFunc: noNextPage[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent](),
					},
					test.apiError,
				)

			cli := &cli{
				apiv2: &auth0.APIV2{Experiments: experimentAPI},
			}

			options, err := cli.experimentPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

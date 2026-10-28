package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/auth0/go-auth0/v3/management"
	managementcore "github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
					ID:            "exp_001",
					Name:          "button-color",
					Status:        management.ExperimentStatusEnumDraft,
					FeatureFlagID: "ff_001",
					IsValid:       false,
				},
				{
					ID:            "exp_002",
					Name:          "checkout-flow",
					Status:        management.ExperimentStatusEnumActive,
					FeatureFlagID: "ff_002",
					IsValid:       true,
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
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
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
				AuthenticationFlow: "authentication",
				AllocationStrategy: management.AllocationStrategyEnumPercentage,
				IsValid:            false,
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv3: &auth0.APIV3{Experiments: experimentAPI},
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
			apiv3: &auth0.APIV3{Experiments: experimentAPI},
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
		assertRequest func(t testing.TB, req *management.CreateExperimentRequestContent)
	}{
		{
			name: "it successfully creates an experiment",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":50,"is_control":true},{"variation_id":"vid_002","weight":50,"is_control":false}]`,
			},
			apiResponse: &management.CreateExperimentResponseContent{
				ID:                 "exp_new",
				Name:               "button-color",
				Status:             management.ExperimentStatusEnumDraft,
				FeatureFlagID:      "ff_001",
				AuthenticationFlow: "authentication",
				AllocationStrategy: management.AllocationStrategyEnumPercentage,
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "button-color")
				assert.Contains(t, out, "exp_new")
			},
		},
		{
			name: "it successfully creates a ramp experiment with levels",
			args: []string{
				"--name", "gradual-rollout",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--levels", "10,25,50,100",
				"--allocations", `[{"variation_id":"vid_001","is_control":true},{"variation_id":"vid_002","is_control":false}]`,
			},
			apiResponse: &management.CreateExperimentResponseContent{
				ID:            "exp_ramp",
				Name:          "gradual-rollout",
				Status:        management.ExperimentStatusEnumDraft,
				FeatureFlagID: "ff_001",
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "gradual-rollout")
				assert.Contains(t, out, "exp_ramp")
			},
		},
		{
			name: "it returns an error when --feature-flag-id is missing",
			args: []string{
				"--name", "button-color",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: `required flag(s) "feature-flag-id" not set`,
		},
		{
			name: "it returns an error when --authentication-flow is missing",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: `required flag(s) "authentication-flow" not set`,
		},
		{
			name: "it returns an error when --allocation-strategy is missing",
			args: []string{
				"--name", "draft-test",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
			},
			expectedError: `required flag(s) "allocation-strategy" not set`,
		},
		{
			name: "it successfully creates a draft with an allocation strategy but no allocations",
			args: []string{
				"--name", "draft-test",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
			},
			apiResponse: &management.CreateExperimentResponseContent{
				ID:            "exp_draft",
				Name:          "draft-test",
				Status:        management.ExperimentStatusEnumDraft,
				FeatureFlagID: "ff_001",
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "exp_draft")
			},
			assertRequest: func(t testing.TB, req *management.CreateExperimentRequestContent) {
				assert.Equal(t, management.AllocationStrategyEnumPercentage, req.AllocationStrategy)
				assert.Empty(t, req.Allocations)
			},
		},
		{
			name: "it returns an error when --allocations is set without --allocation-strategy",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: `required flag(s) "allocation-strategy" not set`,
		},
		{
			name: "it returns an error when --levels is set without --allocation-strategy",
			args: []string{
				"--name", "gradual-rollout",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--levels", "10,50,100",
			},
			expectedError: `required flag(s) "allocation-strategy" not set`,
		},
		{
			name: "it returns an error when --default-config is invalid",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--default-config", "bogus",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: "must be tenant or flag",
		},
		{
			name: "it returns an error when --levels is not strictly increasing",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--levels", "50,50,100",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: "strictly increasing",
		},
		{
			name: "it returns an error when --levels is out of range",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--levels", "10,150",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: "must be between 0 and 100",
		},
		{
			name: "it returns an error when a ramp experiment's allocations have weights",
			args: []string{
				"--name", "gradual-rollout",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--levels", "10,50,100",
				"--allocations", `[{"variation_id":"vid_001","weight":50,"is_control":true},{"variation_id":"vid_002","weight":50,"is_control":false}]`,
			},
			expectedError: "ramp experiments don't use allocation weights",
		},
		{
			name: "it returns an error when ramp levels are used with the segment strategy",
			args: []string{
				"--name", "gradual-rollout",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "segment",
				"--levels", "10,50,100",
				"--allocations", `[{"variation_id":"vid_001","is_control":true,"is_fallback":true}]`,
			},
			expectedError: "ramp levels require --allocation-strategy percentage",
		},
		{
			name: "it returns an error when --allocations is empty",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", "[]",
			},
			expectedError: "--allocations must contain at least one variation",
		},
		{
			name: "it returns an error when --allocations is invalid JSON",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", "not-json",
			},
			expectedError: "invalid JSON for --allocations",
		},
		{
			name: "it returns an error when --allocations weight is out of range",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":-1,"is_control":true}]`,
			},
			expectedError: "must be a whole number between 0 and 100",
		},
		{
			name: "it returns an error when --allocations weight is fractional",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":50.5,"is_control":true}]`,
			},
			expectedError: "cannot unmarshal number 50.5",
		},
		{
			name: "it returns an error when --description is too short",
			args: []string{
				"--name", "button-color",
				"--description", "ab",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
			},
			expectedError: "--description must be at least 3 characters long",
		},
		{
			name: "it returns an error if the API call fails",
			args: []string{
				"--name", "button-color",
				"--feature-flag-id", "ff_001",
				"--authentication-flow", "authentication",
				"--allocation-strategy", "percentage",
				"--allocations", `[{"variation_id":"vid_001","weight":100,"is_control":true}]`,
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
					DoAndReturn(func(_ context.Context, req *management.CreateExperimentRequestContent, _ ...option.RequestOption) (*management.CreateExperimentResponseContent, error) {
						if test.assertRequest != nil {
							test.assertRequest(t, req)
						}
						return test.apiResponse, test.apiError
					})
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
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

// overLimitAllocationsJSON builds a valid JSON array of 21 allocations, one
// past the maximum an experiment accepts, for testing the count validation.
func overLimitAllocationsJSON(t *testing.T) string {
	t.Helper()
	var items []*management.AllocationRequestItem
	for i := range maxExperimentAllocations + 1 {
		w := 5
		items = append(items, &management.AllocationRequestItem{
			VariationID: "v" + strconv.Itoa(i),
			Weight:      &w,
		})
	}
	b, err := json.Marshal(items)
	assert.NoError(t, err)
	return string(b)
}

func TestExperimentsUpdateCmd(t *testing.T) {
	const expID = "exp_abc123"

	// The mocked Get returns this name; update cases diff against it.
	const currentName = "old-name"

	weight := 50
	percentage := management.AllocationStrategyEnum("percentage")
	segment := management.AllocationStrategyEnum("segment")

	tests := []struct {
		name          string
		args          []string
		current       *management.GetExperimentResponseContent
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
			name: "it returns an error when setting levels leaves weighted allocations in place on a draft",
			args: []string{expID, "--levels", "10,50,100"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, Status: management.ExperimentStatusEnumDraft, AllocationStrategy: percentage,
				Allocations: []*management.AllocationItem{{Weight: &weight}, {Weight: &weight}},
			},
			expectedError: "also pass --allocations without \"weight\"",
		},
		{
			// Levels can only change on a draft, so the API reports that rule
			// rather than the CLI asking for unweighted allocations.
			name: "it leaves setting levels on a non-draft experiment to the API",
			args: []string{expID, "--levels", "10,50,100"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, Status: management.ExperimentStatusEnumActive, AllocationStrategy: percentage,
				Allocations: []*management.AllocationItem{{Weight: &weight}, {Weight: &weight}},
			},
			apiError:      errors.New("409 Conflict: experiment is not editable"),
			expectedError: `failed to update experiment "exp_abc123": 409 Conflict: experiment is not editable`,
		},
		{
			name: "it successfully sets levels together with unweighted allocations",
			args: []string{expID, "--levels", "10,50,100", "--allocations", `[{"variation_id":"v1","is_control":true},{"variation_id":"v2","is_control":false}]`},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, AllocationStrategy: percentage,
				Allocations: []*management.AllocationItem{{Weight: &weight}, {Weight: &weight}},
			},
			apiResponse: &management.UpdateExperimentResponseContent{ID: expID, Name: currentName},
		},
		{
			name: "it returns an error when a ramp experiment's new allocations have weights",
			args: []string{expID, "--allocations", `[{"variation_id":"v1","weight":100,"is_control":true}]`},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, AllocationStrategy: percentage, Levels: []int{10, 50, 100},
			},
			expectedError: "ramp experiments don't use allocation weights",
		},
		{
			name: "it returns an error when setting levels on a segment experiment",
			args: []string{expID, "--levels", "10,50,100"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, AllocationStrategy: segment,
			},
			expectedError: "ramp levels require the percentage allocation strategy",
		},
		{
			name: "it returns an error when setting levels on an experiment without a strategy",
			args: []string{expID, "--levels", "10,50,100"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName,
			},
			expectedError: "but this experiment has none",
		},
		{
			name: "it successfully updates the ramp levels",
			args: []string{expID, "--levels", "10,50,100"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, AllocationStrategy: percentage,
			},
			apiResponse: &management.UpdateExperimentResponseContent{ID: expID, Name: currentName},
		},
		{
			name:        "it successfully updates allocations with a zero weight",
			args:        []string{expID, "--allocations", `[{"variation_id":"v1","weight":100,"is_control":true},{"variation_id":"v2","weight":0,"is_control":false}]`},
			apiResponse: &management.UpdateExperimentResponseContent{ID: expID, Name: currentName},
		},
		{
			name:          "it returns an error when no flags are provided",
			args:          []string{expID},
			expectedError: "nothing to update",
		},
		{
			name:          "it returns an error when --default-config is invalid",
			args:          []string{expID, "--default-config", "bogus"},
			expectedError: "must be tenant or flag",
		},
		{
			name: "it returns an error when --levels is not strictly increasing",
			args: []string{expID, "--levels", "100,50"},
			current: &management.GetExperimentResponseContent{
				ID: expID, Name: currentName, AllocationStrategy: percentage,
			},
			expectedError: "strictly increasing",
		},
		{
			name:          "it returns an error when --allocations is invalid JSON",
			args:          []string{expID, "--allocations", "not-json"},
			expectedError: "invalid JSON for --allocations",
		},
		{
			name:          "it returns an error when --allocations is empty",
			args:          []string{expID, "--allocations", "[]"},
			expectedError: "--allocations must contain at least one variation",
		},
		{
			name:          "it returns an error when --allocations exceeds the maximum",
			args:          []string{expID, "--allocations", overLimitAllocationsJSON(t)},
			expectedError: "an experiment accepts at most 20",
		},
		{
			name:          "it returns an error when an allocation weight is out of range",
			args:          []string{expID, "--allocations", `[{"variation_id":"v1","weight":150,"is_control":true}]`},
			expectedError: "must be a whole number between 0 and 100",
		},
		{
			name:        "it successfully updates the status",
			args:        []string{expID, "--status", "paused"},
			apiResponse: &management.UpdateExperimentResponseContent{ID: expID, Name: currentName},
		},
		{
			name:          "it returns an error when --status is invalid",
			args:          []string{expID, "--status", "bogus"},
			expectedError: `invalid status "bogus": must be one of active, paused, completed, archived`,
		},
		{
			name:          "it returns an error when --description is too short",
			args:          []string{expID, "--description", "ab"},
			expectedError: "--description must be at least 3 characters long",
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
			// Update always reads the current experiment first to pre-fill values
			// and compute the diff.
			current := test.current
			if current == nil {
				current = &management.GetExperimentResponseContent{ID: expID, Name: currentName}
			}
			experimentAPI.EXPECT().
				Get(gomock.Any(), expID).
				Return(current, nil)
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
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
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

func TestExperimentsUpdateCmdRendersFullResponse(t *testing.T) {
	const expID = "exp_abc123"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	experimentAPI := mock.NewMockExperimentsAPI(ctrl)
	experimentAPI.EXPECT().
		Get(gomock.Any(), expID).
		Return(&management.GetExperimentResponseContent{ID: expID, Name: "old-name"}, nil)
	experimentAPI.EXPECT().
		Update(gomock.Any(), expID, gomock.Any()).
		Return(&management.UpdateExperimentResponseContent{
			ID:                 expID,
			Name:               "new-name",
			Status:             management.ExperimentStatusEnumDraft,
			FeatureFlagID:      "ff_001",
			AuthenticationFlow: "authentication",
			AllocationStrategy: management.AllocationStrategyEnumPercentage,
		}, nil)

	stdout := &bytes.Buffer{}
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
		apiv3: &auth0.APIV3{Experiments: experimentAPI},
	}

	cmd := updateExperimentCmd(cli)
	cmd.SetArgs([]string{expID, "--name", "new-name"})
	err := cmd.Execute()

	assert.NoError(t, err)
	// The update output should include the fields returned by the API, not just
	// the name/status. This guards against the update view dropping fields.
	out := stdout.String()
	assert.Contains(t, out, "ff_001")
	assert.Contains(t, out, "authentication")
	assert.Contains(t, out, "percentage")
}

func TestExperimentsDeleteCmd(t *testing.T) {
	const expID = "exp_abc123"

	tests := []struct {
		name          string
		getErr        error
		deleteErr     error
		expectDelete  bool
		expectedError string
	}{
		{
			name:         "it successfully deletes an experiment with --force",
			expectDelete: true,
		},
		{
			// Delete returns 204 for an unknown ID, so the 404 comes from the lookup.
			name:          "it returns the not-found error for an unknown experiment",
			getErr:        errors.New("404 Not Found: experiment_not_found"),
			expectedError: `failed to delete experiment "exp_abc123": 404 Not Found: experiment_not_found`,
		},
		{
			name:          "it returns an error if the delete call fails",
			deleteErr:     errors.New("500 Internal Server Error"),
			expectDelete:  true,
			expectedError: `failed to delete experiment "exp_abc123": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			experimentAPI.EXPECT().
				Get(gomock.Any(), expID).
				Return(&management.GetExperimentResponseContent{ID: expID}, test.getErr)
			if test.expectDelete {
				experimentAPI.EXPECT().
					Delete(gomock.Any(), expID).
					Return(test.deleteErr)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
				force: true,
			}

			cmd := deleteExperimentCmd(cli)
			cmd.SetArgs([]string{expID})
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
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
			apiv3: &auth0.APIV3{Experiments: experimentAPI},
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
			apiv3: &auth0.APIV3{Experiments: experimentAPI},
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
			apiv3: &auth0.APIV3{Experiments: experimentAPI},
		}

		cmd := validateExperimentCmd(cli)
		cmd.SetArgs([]string{expID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to validate experiment "exp_abc123": 500 Internal Server Error`)
	})
}

func TestExperimentsAdvanceRampCmd(t *testing.T) {
	const expID = "exp_abc123"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.AdvanceRampResponseContent
		apiError      error
		expectCall    bool
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name:       "it successfully advances the ramp to the next level",
			args:       []string{expID, "--target-level", "25"},
			expectCall: true,
			apiResponse: &management.AdvanceRampResponseContent{
				ExperimentID: expID,
				FromLevel:    10,
				ToLevel:      25,
				CurrentLevel: 25,
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, expID)
				assert.Contains(t, out, "25")
			},
		},
		{
			name:          "it returns an error when --target-level is missing",
			args:          []string{expID},
			expectedError: "--target-level is required",
		},
		{
			name:          "it returns an error when --target-level is out of range",
			args:          []string{expID, "--target-level", "150"},
			expectedError: "must be between 0 and 100",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{expID, "--target-level", "25"},
			expectCall:    true,
			apiError:      errors.New("409 Conflict"),
			expectedError: `failed to advance ramp for experiment "exp_abc123": 409 Conflict`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			experimentAPI := mock.NewMockExperimentsAPI(ctrl)
			if test.expectCall {
				experimentAPI.EXPECT().
					AdvanceRamp(gomock.Any(), expID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
			}

			cmd := advanceRampExperimentCmd(cli)
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

func TestParseLevels(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		expected      []int
		expectedError string
	}{
		{name: "single level", raw: "50", expected: []int{50}},
		{name: "strictly increasing", raw: "10,25,50,100", expected: []int{10, 25, 50, 100}},
		{name: "trims whitespace", raw: " 10 , 20 ", expected: []int{10, 20}},
		{name: "boundaries 0 and 100", raw: "0,100", expected: []int{0, 100}},
		{name: "rejects non-increasing", raw: "50,50", expectedError: "strictly increasing"},
		{name: "rejects decreasing", raw: "100,10", expectedError: "strictly increasing"},
		{name: "rejects out of range", raw: "10,150", expectedError: "must be between 0 and 100"},
		{name: "rejects negative", raw: "-5,10", expectedError: "must be between 0 and 100"},
		{name: "rejects non-numeric", raw: "10,abc", expectedError: "must be a whole number"},
		{name: "rejects empty", raw: "", expectedError: "at least one"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			levels, err := parseLevels(test.raw)
			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, levels)
			}
		})
	}
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
				apiv3: &auth0.APIV3{Experiments: experimentAPI},
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

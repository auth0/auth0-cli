package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	management "github.com/auth0/go-auth0/v3/management"
	managementcore "github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

// noNextPage returns a NextPageFunc that always returns ErrNoPages.
func noNextPage[C comparable, T any, R any]() func(context.Context) (*managementcore.Page[C, T, R], error) {
	return func(_ context.Context) (*managementcore.Page[C, T, R], error) {
		return nil, managementcore.ErrNoPages
	}
}

func TestFeatureFlagsListCmd(t *testing.T) {
	tests := []struct {
		name          string
		flags         []*management.FeatureFlag
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully lists feature flags",
			flags: []*management.FeatureFlag{
				{
					ID:     "ff_001",
					Name:   "dark-mode",
					Type:   management.FeatureFlagTypeEnumSelf,
					Status: management.FeatureFlagStatusEnumActive,
				},
				{
					ID:     "ff_002",
					Name:   "checkout-flow",
					Type:   management.FeatureFlagTypeEnumAuth0,
					Status: management.FeatureFlagStatusEnumDraft,
				},
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "dark-mode")
				assert.Contains(t, out, "ff_001")
				assert.Contains(t, out, "checkout-flow")
				assert.Contains(t, out, "ff_002")
			},
		},
		{
			name:  "it displays an empty state when there are no feature flags",
			flags: []*management.FeatureFlag{},
			assertOutput: func(t testing.TB, out string) {
				assert.Empty(t, out)
			},
		},
		{
			name:          "it returns an error if the API call fails",
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: "failed to list feature flags: 500 Internal Server Error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
			featureFlagAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(
					&managementcore.Page[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent]{
						Results:      test.flags,
						NextPageFunc: noNextPage[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent](),
					},
					test.apiError,
				)

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			}

			cmd := listFeatureFlagsCmd(cli)
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

func TestFeatureFlagsShowCmd(t *testing.T) {
	const flagID = "ff_abc123"

	t.Run("it successfully shows a feature flag", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
		featureFlagAPI.EXPECT().
			Get(gomock.Any(), flagID).
			Return(&management.GetFeatureFlagResponseContent{
				ID:     flagID,
				Name:   "dark-mode",
				Type:   management.FeatureFlagTypeEnumSelf,
				Status: management.FeatureFlagStatusEnumActive,
			}, nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
		}

		cmd := showFeatureFlagCmd(cli)
		cmd.SetArgs([]string{flagID})
		err := cmd.Execute()

		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "dark-mode")
		assert.Contains(t, stdout.String(), flagID)
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
		featureFlagAPI.EXPECT().
			Get(gomock.Any(), flagID).
			Return(nil, errors.New("404 Not Found"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
		}

		cmd := showFeatureFlagCmd(cli)
		cmd.SetArgs([]string{flagID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to get feature flag "ff_abc123": 404 Not Found`)
	})
}

func TestFeatureFlagsCreateCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.CreateFeatureFlagResponseContent
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully creates a feature flag",
			args: []string{
				"--name", "dark-mode",
				"--parameters", `{"enabled":{"type":"boolean","value":false}}`,
			},
			apiResponse: &management.CreateFeatureFlagResponseContent{
				ID:     "ff_new123",
				Name:   "dark-mode",
				Status: management.FeatureFlagStatusEnumDraft,
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "dark-mode")
				assert.Contains(t, out, "ff_new123")
			},
		},
		{
			name: "it returns an error when --parameters is empty",
			args: []string{
				"--name", "dark-mode",
				"--parameters", "",
			},
			expectedError: "--parameters is required",
		},
		{
			name: "it returns an error when --parameters is invalid JSON",
			args: []string{
				"--name", "dark-mode",
				"--parameters", "not-json",
			},
			expectedError: "invalid JSON for --parameters",
		},
		{
			name: "it returns an error if the API call fails",
			args: []string{
				"--name", "dark-mode",
				"--parameters", `{"enabled":{"type":"boolean","value":false}}`,
			},
			apiError:      errors.New("400 Bad Request"),
			expectedError: "failed to create feature flag: 400 Bad Request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
			if test.apiResponse != nil || test.apiError != nil {
				featureFlagAPI.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			}

			cmd := createFeatureFlagCmd(cli)
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

func TestFeatureFlagsUpdateCmd(t *testing.T) {
	const flagID = "ff_abc123"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.UpdateFeatureFlagResponseContent
		apiError      error
		expectedError string
	}{
		{
			name: "it successfully updates the name",
			args: []string{flagID, "--name", "new-name"},
			apiResponse: &management.UpdateFeatureFlagResponseContent{
				ID:   flagID,
				Name: "new-name",
			},
		},
		{
			name:          "it returns an error when no flags are provided",
			args:          []string{flagID},
			expectedError: "nothing to update",
		},
		{
			name:          "it returns an error when --parameters is invalid JSON",
			args:          []string{flagID, "--parameters", "not-json"},
			expectedError: "invalid JSON for --parameters",
		},
		{
			name: "it successfully updates the status",
			args: []string{flagID, "--status", "active"},
			apiResponse: &management.UpdateFeatureFlagResponseContent{
				ID:   flagID,
				Name: "old-name",
			},
		},
		{
			name:          "it returns an error when --status is invalid",
			args:          []string{flagID, "--status", "bogus"},
			expectedError: `invalid status "bogus": must be one of active, archived`,
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{flagID, "--name", "new-name"},
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: `failed to update feature flag "ff_abc123": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
			// Update reads the current flag first to pre-fill values and diff.
			featureFlagAPI.EXPECT().
				Get(gomock.Any(), flagID).
				Return(&management.GetFeatureFlagResponseContent{ID: flagID, Name: "old-name"}, nil)
			if test.apiResponse != nil || test.apiError != nil {
				featureFlagAPI.EXPECT().
					Update(gomock.Any(), flagID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			}

			cmd := updateFeatureFlagCmd(cli)
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

func TestFeatureFlagsUpdateCmdRendersFullResponse(t *testing.T) {
	const flagID = "ff_abc123"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
	featureFlagAPI.EXPECT().
		Get(gomock.Any(), flagID).
		Return(&management.GetFeatureFlagResponseContent{ID: flagID, Name: "old-name"}, nil)
	featureFlagAPI.EXPECT().
		Update(gomock.Any(), flagID, gomock.Any()).
		Return(&management.UpdateFeatureFlagResponseContent{
			ID:     flagID,
			Name:   "new-name",
			Type:   management.FeatureFlagTypeEnumSelf,
			Status: management.FeatureFlagStatusEnumDraft,
		}, nil)

	stdout := &bytes.Buffer{}
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
		apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
	}

	cmd := updateFeatureFlagCmd(cli)
	cmd.SetArgs([]string{flagID, "--name", "new-name"})
	err := cmd.Execute()

	assert.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "new-name")
	assert.Contains(t, out, "self")
}

func TestFeatureFlagsUpdateCmdSkipsUnchangedParameters(t *testing.T) {
	const flagID = "ff_abc123"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Build a flag with parameters, then pass --parameters equal to the current
	// value. The command should diff it out and NOT send parameters, but since
	// nothing else changed it should report "nothing to update".
	params := management.FeatureFlagConfigParams{
		"color": &management.FeatureFlagConfigParam{Type: "string", Value: "blue"},
	}
	current := &management.GetFeatureFlagResponseContent{ID: flagID, Name: "old-name", Parameters: &params}
	currentJSON := marshalToJSON(current.GetParameters())

	featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
	featureFlagAPI.EXPECT().
		Get(gomock.Any(), flagID).
		Return(current, nil)
	// Update must NOT be called — no EXPECT() for it.

	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
	}

	cmd := updateFeatureFlagCmd(cli)
	cmd.SetArgs([]string{flagID, "--parameters", currentJSON})
	err := cmd.Execute()

	assert.ErrorContains(t, err, "nothing to update")
}

func TestFeatureFlagsUpdateCmdClearsDescription(t *testing.T) {
	const flagID = "ff_abc123"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	description := "old description"
	featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
	featureFlagAPI.EXPECT().
		Get(gomock.Any(), flagID).
		Return(&management.GetFeatureFlagResponseContent{ID: flagID, Name: "old-name", Description: &description}, nil)

	// The API rejects an empty description, so clearing it must send null.
	var sent []byte
	featureFlagAPI.EXPECT().
		Update(gomock.Any(), flagID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, req *management.UpdateFeatureFlagRequestContent, _ ...option.RequestOption) (*management.UpdateFeatureFlagResponseContent, error) {
			var err error
			sent, err = json.Marshal(req)
			return &management.UpdateFeatureFlagResponseContent{ID: flagID, Name: "old-name"}, err
		})

	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
	}

	cmd := updateFeatureFlagCmd(cli)
	cmd.SetArgs([]string{flagID, "--description", ""})
	err := cmd.Execute()

	assert.NoError(t, err)
	assert.JSONEq(t, `{"description":null}`, string(sent))
}

func TestFeatureFlagsDeleteCmd(t *testing.T) {
	const flagID = "ff_abc123"

	t.Run("it successfully deletes a feature flag with --force", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
		featureFlagAPI.EXPECT().
			Delete(gomock.Any(), flagID).
			Return(nil)

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			force: true,
		}

		cmd := deleteFeatureFlagCmd(cli)
		cmd.SetArgs([]string{flagID})
		err := cmd.Execute()

		assert.NoError(t, err)
	})

	t.Run("it returns an error if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
		featureFlagAPI.EXPECT().
			Delete(gomock.Any(), flagID).
			Return(errors.New("500 Internal Server Error"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			force: true,
		}

		cmd := deleteFeatureFlagCmd(cli)
		cmd.SetArgs([]string{flagID})
		err := cmd.Execute()

		assert.EqualError(t, err, `failed to delete feature flag "ff_abc123": 500 Internal Server Error`)
	})
}

func TestFeatureFlagPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		flags        []*management.FeatureFlag
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "it returns picker options for each feature flag",
			flags: []*management.FeatureFlag{
				{ID: "ff_001", Name: "dark-mode"},
				{ID: "ff_002", Name: "checkout-flow"},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "ff_001", options[0].value)
				assert.Equal(t, "ff_002", options[1].value)
				assert.Contains(t, options[0].label, "dark-mode")
				assert.Contains(t, options[0].label, "ff_001")
				assert.Contains(t, options[1].label, "checkout-flow")
				assert.Contains(t, options[1].label, "ff_002")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:  "it returns an error when there are no feature flags",
			flags: []*management.FeatureFlag{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "no feature flags available")
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

			featureFlagAPI := mock.NewMockFeatureFlagsAPI(ctrl)
			featureFlagAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(
					&managementcore.Page[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent]{
						Results:      test.flags,
						NextPageFunc: noNextPage[*string, *management.FeatureFlag, *management.ListFeatureFlagsResponseContent](),
					},
					test.apiError,
				)

			cli := &cli{
				apiv3: &auth0.APIV3{FeatureFlags: featureFlagAPI},
			}

			options, err := cli.featureFlagPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestVariationPickerOptions(t *testing.T) {
	const flagID = "ff_abc123"

	tests := []struct {
		name         string
		variations   []*management.Variation
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "it returns picker options for each variation",
			variations: []*management.Variation{
				{ID: "vid_001", Name: "control"},
				{ID: "vid_002", Name: "treatment"},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "vid_001", options[0].value)
				assert.Equal(t, "vid_002", options[1].value)
				assert.Contains(t, options[0].label, "control")
				assert.Contains(t, options[0].label, "vid_001")
				assert.Contains(t, options[1].label, "treatment")
				assert.Contains(t, options[1].label, "vid_002")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:       "it returns an error when there are no variations",
			variations: []*management.Variation{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "no variations for feature flag")
				assert.ErrorContains(t, err, flagID)
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

			variationsAPI := mock.NewMockVariationsAPI(ctrl)
			variationsAPI.EXPECT().
				List(gomock.Any(), flagID).
				Return(&management.ListVariationsResponseContent{
					Variations: test.variations,
				}, test.apiError)

			cli := &cli{
				apiv3: &auth0.APIV3{Variations: variationsAPI},
			}

			pickerFn := cli.variationPickerOptions(flagID)
			options, err := pickerFn(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestVariationsListCmd(t *testing.T) {
	const flagID = "ff_abc123"

	tests := []struct {
		name          string
		variations    []*management.Variation
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully lists variations",
			variations: []*management.Variation{
				{ID: "vid_001", Name: "control"},
				{ID: "vid_002", Name: "treatment"},
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "control")
				assert.Contains(t, out, "vid_001")
				assert.Contains(t, out, "treatment")
				assert.Contains(t, out, "vid_002")
			},
		},
		{
			name:       "it displays an empty state when there are no variations",
			variations: []*management.Variation{},
			assertOutput: func(t testing.TB, out string) {
				assert.Empty(t, out)
			},
		},
		{
			name:          "it returns an error if the API call fails",
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: "failed to list variations",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			variationsAPI := mock.NewMockVariationsAPI(ctrl)
			variationsAPI.EXPECT().
				List(gomock.Any(), flagID).
				Return(&management.ListVariationsResponseContent{
					Variations: test.variations,
				}, test.apiError)

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{Variations: variationsAPI},
			}

			cmd := listVariationsCmd(cli)
			cmd.SetArgs([]string{flagID})
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

func TestVariationsCreateCmd(t *testing.T) {
	const flagID = "ff_abc123"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.CreateVariationResponseContent
		apiError      error
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name: "it successfully creates a variation",
			args: []string{
				flagID,
				"--name", "treatment",
				"--overrides", `{"color":"red"}`,
			},
			apiResponse: &management.CreateVariationResponseContent{
				ID:            "vid_new",
				FeatureFlagID: flagID,
				Name:          "treatment",
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "treatment")
				assert.Contains(t, out, "vid_new")
			},
		},
		{
			name: "it returns an error when --overrides is empty",
			args: []string{
				flagID,
				"--name", "treatment",
				"--overrides", "",
			},
			expectedError: "--overrides is required",
		},
		{
			name: "it returns an error when --overrides is invalid JSON",
			args: []string{
				flagID,
				"--name", "treatment",
				"--overrides", "not-json",
			},
			expectedError: "invalid JSON for --overrides",
		},
		{
			name: "it returns an error if the API call fails",
			args: []string{
				flagID,
				"--name", "treatment",
				"--overrides", `{"color":"red"}`,
			},
			apiError:      errors.New("400 Bad Request"),
			expectedError: "failed to create variation: 400 Bad Request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			variationsAPI := mock.NewMockVariationsAPI(ctrl)
			if test.apiResponse != nil || test.apiError != nil {
				variationsAPI.EXPECT().
					Create(gomock.Any(), flagID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{Variations: variationsAPI},
			}

			cmd := createVariationCmd(cli)
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

func TestVariationsUpdateCmd(t *testing.T) {
	const flagID = "ff_abc123"
	const varID = "vid_001"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.UpdateVariationResponseContent
		apiError      error
		expectedError string
	}{
		{
			name:        "it successfully updates the name",
			args:        []string{flagID, varID, "--name", "new-treatment"},
			apiResponse: &management.UpdateVariationResponseContent{ID: varID, Name: "new-treatment"},
		},
		{
			name:          "it returns an error when no flags are provided",
			args:          []string{flagID, varID},
			expectedError: "nothing to update",
		},
		{
			name:          "it returns an error when --overrides is invalid JSON",
			args:          []string{flagID, varID, "--overrides", "not-json"},
			expectedError: "invalid JSON for --overrides",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{flagID, varID, "--name", "new-treatment"},
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: `failed to update variation "vid_001": 500 Internal Server Error`,
		},
		{
			name:        "it successfully adds and changes override keys",
			args:        []string{flagID, varID, "--overrides", `{"color":"blue","size":2}`},
			apiResponse: &management.UpdateVariationResponseContent{ID: varID, Name: "old-treatment"},
		},
		{
			name:          "it returns an error when override keys are removed",
			args:          []string{flagID, varID, "--overrides", `{"size":2}`},
			expectedError: "override keys can't be removed from an existing variation (color)",
		},
		{
			name:          "it returns an error when overrides are reset to the control",
			args:          []string{flagID, varID, "--overrides", `{}`},
			expectedError: "override keys can't be removed from an existing variation (color)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			variationsAPI := mock.NewMockVariationsAPI(ctrl)
			// Update reads the current variation first to pre-fill values and diff.
			variationsAPI.EXPECT().
				Get(gomock.Any(), flagID, varID).
				Return(&management.GetVariationResponseContent{
					ID:        varID,
					Name:      "old-treatment",
					Overrides: management.VariationOverridesMap{"color": "red"},
				}, nil)
			if test.apiResponse != nil || test.apiError != nil {
				variationsAPI.EXPECT().
					Update(gomock.Any(), flagID, varID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3: &auth0.APIV3{Variations: variationsAPI},
			}

			cmd := updateVariationCmd(cli)
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

func TestVariationsUpdateCmdRendersFullResponse(t *testing.T) {
	const flagID = "ff_abc123"
	const varID = "vid_001"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	variationsAPI := mock.NewMockVariationsAPI(ctrl)
	variationsAPI.EXPECT().
		Get(gomock.Any(), flagID, varID).
		Return(&management.GetVariationResponseContent{ID: varID, Name: "old-treatment"}, nil)
	variationsAPI.EXPECT().
		Update(gomock.Any(), flagID, varID, gomock.Any()).
		Return(&management.UpdateVariationResponseContent{
			ID:            varID,
			FeatureFlagID: flagID,
			Name:          "new-treatment",
		}, nil)

	stdout := &bytes.Buffer{}
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
		apiv3: &auth0.APIV3{Variations: variationsAPI},
	}

	cmd := updateVariationCmd(cli)
	cmd.SetArgs([]string{flagID, varID, "--name", "new-treatment"})
	err := cmd.Execute()

	assert.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "new-treatment")
	// FeatureFlagID should be rendered on the update view.
	assert.Contains(t, out, flagID)
}

func TestVariationsDeleteCmd(t *testing.T) {
	const flagID = "ff_abc123"

	tests := []struct {
		name          string
		args          []string
		agentMode     bool
		deletes       map[string]error // variation ID -> Delete result
		expectedError []string
	}{
		{
			name:    "it deletes a single variation",
			args:    []string{flagID, "var_1", "--force"},
			deletes: map[string]error{"var_1": nil},
		},
		{
			name:    "it deletes multiple variations",
			args:    []string{flagID, "var_1", "var_2", "var_3", "--force"},
			deletes: map[string]error{"var_1": nil, "var_2": nil, "var_3": nil},
		},
		{
			name: "it continues past a failed delete and reports it",
			args: []string{flagID, "var_1", "var_2", "var_3", "--force"},
			deletes: map[string]error{
				"var_1": nil,
				"var_2": errors.New("500 Internal Server Error"),
				"var_3": nil,
			},
			expectedError: []string{`failed to delete variation "var_2"`, "500 Internal Server Error"},
		},
		{
			// The API returns 204 for an unknown variation, so the CLI does too.
			name:    "it succeeds for an unknown variation, as the API does",
			args:    []string{flagID, "var_missing", "--force"},
			deletes: map[string]error{"var_missing": nil},
		},
		{
			// The API returns 404 when the feature flag itself doesn't exist.
			name:          "it reports the API's not-found error for an unknown feature flag",
			args:          []string{flagID, "var_1", "--force"},
			deletes:       map[string]error{"var_1": errors.New("404 Not Found: Feature flag not found")},
			expectedError: []string{`failed to delete variation "var_1"`, "404 Not Found"},
		},
		{
			name:          "it requires --force in agent mode",
			args:          []string{flagID, "var_1"},
			agentMode:     true,
			expectedError: []string{errDestructiveNoConfirm.Error()},
		},
		{
			name:      "it deletes in agent mode when --force is set",
			args:      []string{flagID, "var_1", "--force"},
			agentMode: true,
			deletes:   map[string]error{"var_1": nil},
		},
		{
			name:          "it returns an error when no variation ID is given non-interactively",
			args:          []string{flagID},
			expectedError: []string{"missing a required argument"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			variationsAPI := mock.NewMockVariationsAPI(ctrl)
			for id, err := range test.deletes {
				variationsAPI.EXPECT().Delete(gomock.Any(), flagID, id).Return(err)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3:     &auth0.APIV3{Variations: variationsAPI},
				agentMode: test.agentMode,
			}

			cmd := deleteVariationCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if len(test.expectedError) == 0 {
				assert.NoError(t, err)
				return
			}
			for _, want := range test.expectedError {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

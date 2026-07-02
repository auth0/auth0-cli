package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"

	management "github.com/auth0/go-auth0/v2/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestSegmentAttributesAndConditions(t *testing.T) {
	// These lists are derived from the SDK types via reflection. Assert the
	// expected members so an unexpected SDK change is caught here rather than
	// silently altering the CLI's help and validation.
	assert.ElementsMatch(t, []string{
		"client_id", "connection", "connection_type", "organization_id",
		"domain", "device_type", "browser", "platform", "user_agent",
		"country", "region",
	}, segmentAttributes)

	assert.ElementsMatch(t, []string{
		"contains", "starts_with", "ends_with", "exists",
	}, segmentConditions)
}

func TestParseSegmentRules_Valid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "single match with operator",
			raw:  `[{"match":{"domain":{"ends_with":["example.com"]}}}]`,
		},
		{
			name: "plain list is an exact match",
			raw:  `[{"match":{"country":["US"]}}]`,
		},
		{
			name: "multiple attributes in one match",
			raw:  `[{"match":{"country":["US"],"browser":{"contains":["Chrome"]}}}]`,
		},
		{
			name: "match and not_match together",
			raw:  `[{"match":{"domain":{"ends_with":["example.com"]}},"not_match":{"country":["US"]}}]`,
		},
		{
			name: "not_match only",
			raw:  `[{"not_match":{"connection":["google-oauth2"]}}]`,
		},
		{
			name: "exists condition",
			raw:  `[{"match":{"organization_id":{"exists":true}}}]`,
		},
		{
			name: "multiple rules",
			raw:  `[{"match":{"domain":{"contains":["a.com"]}}},{"not_match":{"country":["US"]}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := parseSegmentRules(tt.raw)
			assert.NoError(t, err)
			assert.NotEmpty(t, rules)
		})
	}
}

func TestParseSegmentRules_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "operator used as attribute",
			raw:     `[{"match":{"contains":["@example.com"]}}]`,
			wantErr: `rule[0].match.contains: unknown attribute`,
		},
		{
			name:    "unknown condition operator",
			raw:     `[{"match":{"domain":{"has":["x"]}}}]`,
			wantErr: `rule[0].match.domain.has: unknown condition`,
		},
		{
			name:    "unknown top-level key",
			raw:     `[{"foo":{"domain":["x"]}}]`,
			wantErr: `rule[0].foo: unknown key`,
		},
		{
			name:    "unknown attribute under not_match",
			raw:     `[{"not_match":{"bogus":["x"]}}]`,
			wantErr: `rule[0].not_match.bogus: unknown attribute`,
		},
		{
			name:    "error reports correct rule index",
			raw:     `[{"match":{"domain":{"contains":["a.com"]}}},{"match":{"nope":["x"]}}]`,
			wantErr: `rule[1].match.nope: unknown attribute`,
		},
		{
			name:    "invalid json",
			raw:     `not json`,
			wantErr: `invalid JSON for --rules`,
		},
		{
			name:    "object instead of array",
			raw:     `{"match":{"domain":["x"]}}`,
			wantErr: `invalid JSON for --rules`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSegmentRules(tt.raw)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSegmentsUpdateCmd(t *testing.T) {
	const segID = "seg_abc123"

	// The mocked Get returns this name; update cases diff against it.
	const currentName = "old-segment"

	tests := []struct {
		name          string
		args          []string
		apiResponse   *management.UpdateSegmentResponseContent
		apiError      error
		expectedError string
	}{
		{
			name:        "it successfully updates the name",
			args:        []string{segID, "--name", "new-segment"},
			apiResponse: &management.UpdateSegmentResponseContent{ID: segID, Name: "new-segment"},
		},
		{
			name:          "it returns an error when no flags are provided",
			args:          []string{segID},
			expectedError: "nothing to update",
		},
		{
			name:          "it returns an error when --rules is invalid JSON",
			args:          []string{segID, "--rules", "not-json"},
			expectedError: "invalid JSON for --rules",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{segID, "--name", "new-segment"},
			apiError:      errors.New("500 Internal Server Error"),
			expectedError: `failed to update segment "seg_abc123": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			segmentsAPI := mock.NewMockSegmentsAPI(ctrl)
			// Update reads the current segment first to pre-fill values and diff.
			segmentsAPI.EXPECT().
				Get(gomock.Any(), segID).
				Return(&management.GetSegmentResponseContent{ID: segID, Name: currentName}, nil)
			if test.apiResponse != nil || test.apiError != nil {
				segmentsAPI.EXPECT().
					Update(gomock.Any(), segID, gomock.Any()).
					Return(test.apiResponse, test.apiError)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv2: &auth0.APIV2{Segments: segmentsAPI},
			}

			cmd := updateSegmentCmd(cli)
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

func TestSegmentsUpdateCmdRendersFullResponse(t *testing.T) {
	const segID = "seg_abc123"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	segmentsAPI := mock.NewMockSegmentsAPI(ctrl)
	segmentsAPI.EXPECT().
		Get(gomock.Any(), segID).
		Return(&management.GetSegmentResponseContent{ID: segID, Name: "old-segment"}, nil)
	segmentsAPI.EXPECT().
		Update(gomock.Any(), segID, gomock.Any()).
		Return(&management.UpdateSegmentResponseContent{
			ID:          segID,
			Name:        "new-segment",
			Description: auth0.String("desc"),
			Type:        management.SegmentTypeEnumSelf,
		}, nil)

	stdout := &bytes.Buffer{}
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
		apiv2: &auth0.APIV2{Segments: segmentsAPI},
	}

	cmd := updateSegmentCmd(cli)
	cmd.SetArgs([]string{segID, "--name", "new-segment"})
	err := cmd.Execute()

	assert.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "new-segment")
	// Description should be rendered on the update view.
	assert.Contains(t, out, "desc")
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
		{
			name: "line comments are stripped",
			raw: "// pick US/CA users\n" +
				"[{\"match\":{\"country\":[\"US\",\"CA\"]}}" +
				" // ,{\"not_match\":{\"browser\":[\"safari\"]}}\n" +
				"]",
		},
		{
			name: "block comment is stripped",
			raw:  `[/* block */{"match":{"domain":{"ends_with":["example.com"]}}}]`,
		},
		{
			name: "comment markers inside string values are preserved",
			raw:  `[{"match":{"domain":{"ends_with":["https://example.com"]}}}]`,
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

func TestStripJSONComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no comments is unchanged",
			in:   `[{"match":{"country":["US"]}}]`,
			want: `[{"match":{"country":["US"]}}]`,
		},
		{
			name: "line comment removed, newline kept",
			in:   "a // comment\nb",
			want: "a \nb",
		},
		{
			name: "block comment removed",
			in:   "a /* c */ b",
			want: "a  b",
		},
		{
			name: "double slash inside string preserved",
			in:   `{"url":"https://example.com"}`,
			want: `{"url":"https://example.com"}`,
		},
		{
			name: "slash-star inside string preserved",
			in:   `{"x":"a/*b*/c"}`,
			want: `{"x":"a/*b*/c"}`,
		},
		{
			name: "escaped quote does not end string",
			in:   `{"x":"a\"// b"}`,
			want: `{"x":"a\"// b"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripJSONComments(tt.in))
		})
	}
}

func TestSegmentRulesTemplate(t *testing.T) {
	tmpl := segmentRulesTemplate()

	// The commented header must list every valid attribute and condition so the
	// editor buffer stays a complete reference even as the SDK evolves.
	for _, attr := range segmentAttributes {
		assert.Contains(t, tmpl, attr)
	}
	for _, cond := range segmentConditions {
		assert.Contains(t, tmpl, cond)
	}

	// The template must survive comment-stripping and parse as valid rules, so a
	// user who saves it unedited still creates a valid segment.
	rules, err := parseSegmentRules(tmpl)
	assert.NoError(t, err)
	assert.NotEmpty(t, rules)
}

// TestParseSegmentRules_MarshalsCorrectVariant guards the fix for the SDK's
// SegmentMatchExpression.UnmarshalJSON bug, which collapses every operator object
// into {"contains":null}. ParseSegmentRules must build the correct union variant
// so the value round-trips back to the operator the user wrote.
func TestParseSegmentRules_MarshalsCorrectVariant(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "ends_with",
			raw:  `[{"match":{"domain":{"ends_with":["example.com"]}}}]`,
			want: `[{"match":{"domain":{"ends_with":["example.com"]}}}]`,
		},
		{
			name: "starts_with",
			raw:  `[{"match":{"domain":{"starts_with":["acme"]}}}]`,
			want: `[{"match":{"domain":{"starts_with":["acme"]}}}]`,
		},
		{
			name: "exists",
			raw:  `[{"match":{"organization_id":{"exists":true}}}]`,
			want: `[{"match":{"organization_id":{"exists":true}}}]`,
		},
		{
			name: "contains",
			raw:  `[{"match":{"domain":{"contains":["a.com"]}}}]`,
			want: `[{"match":{"domain":{"contains":["a.com"]}}}]`,
		},
		{
			name: "plain list is an exact match",
			raw:  `[{"match":{"country":["US","CA"]}}]`,
			want: `[{"match":{"country":["US","CA"]}}]`,
		},
		{
			name: "not_match preserves the operator",
			raw:  `[{"not_match":{"browser":{"ends_with":["bot"]}}}]`,
			want: `[{"not_match":{"browser":{"ends_with":["bot"]}}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := parseSegmentRules(tt.raw)
			assert.NoError(t, err)
			got, err := json.Marshal(rules)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
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

func TestSegmentsListCmd(t *testing.T) {
	selfType := management.SegmentTypeFilterEnumSelf

	tests := []struct {
		name         string
		args         []string
		expectedType *management.SegmentTypeFilterEnum
	}{
		{
			name: "it lists segments of every type with their status",
		},
		{
			name:         "it filters segments by --type",
			args:         []string{"--type", "self"},
			expectedType: &selfType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			segmentsAPI := mock.NewMockSegmentsAPI(ctrl)
			segmentsAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, req *management.ListSegmentsRequestParameters, _ ...option.RequestOption) (*auth0.SegmentListPage, error) {
					assert.Equal(t, test.expectedType, req.Type)
					return &auth0.SegmentListPage{
						Results: []*management.Segment{{
							ID:     "seg_001",
							Name:   "eu-users",
							Type:   management.SegmentTypeEnumSelf,
							Status: management.SegmentStatusEnumActive,
						}},
						NextPageFunc: noNextPage[*string, *management.Segment, *management.ListSegmentsResponseContent](),
					}, nil
				})

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				apiv3: &auth0.APIV3{Segments: segmentsAPI},
			}

			cmd := listSegmentsCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			assert.NoError(t, err)
			assert.Contains(t, stdout.String(), "eu-users")
			assert.Contains(t, stdout.String(), "active")
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
			name:        "it successfully updates the status",
			args:        []string{segID, "--status", "archived"},
			apiResponse: &management.UpdateSegmentResponseContent{ID: segID, Name: currentName},
		},
		{
			name:          "it returns an error when --status is invalid",
			args:          []string{segID, "--status", "bogus"},
			expectedError: `invalid status "bogus": must be one of active, archived`,
		},
		{
			name:          "it returns an error when --description is too short",
			args:          []string{segID, "--description", "ab"},
			expectedError: "--description must be at least 3 characters long",
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
				apiv3: &auth0.APIV3{Segments: segmentsAPI},
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
		apiv3: &auth0.APIV3{Segments: segmentsAPI},
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

func TestSegmentsDeleteCmd(t *testing.T) {
	const segID = "seg_abc123"

	tests := []struct {
		name          string
		getErr        error
		deleteErr     error
		expectDelete  bool
		expectedError string
	}{
		{
			name:         "it successfully deletes a segment",
			expectDelete: true,
		},
		{
			// Delete returns 204 for an unknown ID, so the 404 comes from the lookup.
			name:          "it returns the not-found error for an unknown segment",
			getErr:        errors.New("404 Not Found: segment_not_found"),
			expectedError: `failed to delete segment "seg_abc123": 404 Not Found: segment_not_found`,
		},
		{
			name:          "it returns an error if the delete call fails",
			deleteErr:     errors.New("500 Internal Server Error"),
			expectDelete:  true,
			expectedError: `failed to delete segment "seg_abc123": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			segmentsAPI := mock.NewMockSegmentsAPI(ctrl)
			segmentsAPI.EXPECT().Get(gomock.Any(), segID).Return(&management.GetSegmentResponseContent{}, test.getErr)
			if test.expectDelete {
				segmentsAPI.EXPECT().Delete(gomock.Any(), segID).Return(test.deleteErr)
			}

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3: &auth0.APIV3{Segments: segmentsAPI},
			}

			cmd := deleteSegmentCmd(cli)
			cmd.SetArgs([]string{segID, "--force"})
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

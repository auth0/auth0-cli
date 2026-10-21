package cli

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/auth0/go-auth0/v3/management"
	managementcore "github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

// TestExperimentationDeleteRequiresForceInAgentMode guards the agent-mode
// contract shared by every destructive command: without --force, delete must
// refuse rather than remove resources silently. The mocks expect no calls, so
// any API request fails the test.
func TestExperimentationDeleteRequiresForceInAgentMode(t *testing.T) {
	tests := []struct {
		name  string
		build func(*cli) *cobra.Command
		args  []string
	}{
		{name: "experiments delete", build: deleteExperimentCmd, args: []string{"exp_abc123"}},
		{name: "feature-flags delete", build: deleteFeatureFlagCmd, args: []string{"ff_abc123"}},
		{name: "feature-flags variations delete", build: deleteVariationCmd, args: []string{"ff_abc123", "var_1"}},
		{name: "segments delete", build: deleteSegmentCmd, args: []string{"seg_abc123"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				apiv3: &auth0.APIV3{
					Experiments:  mock.NewMockExperimentsAPI(ctrl),
					FeatureFlags: mock.NewMockFeatureFlagsAPI(ctrl),
					Variations:   mock.NewMockVariationsAPI(ctrl),
					Segments:     mock.NewMockSegmentsAPI(ctrl),
				},
				agentMode: true,
			}

			cmd := test.build(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			assert.ErrorIs(t, err, errDestructiveNoConfirm)
		})
	}
}

func TestCollectPages(t *testing.T) {
	type segmentPage = managementcore.Page[*string, *management.Segment, *management.ListSegmentsResponseContent]

	segment := func(id string) *management.Segment { return &management.Segment{ID: id} }
	ids := func(segments []*management.Segment) []string {
		out := make([]string, 0, len(segments))
		for _, s := range segments {
			out = append(out, s.GetID())
		}
		return out
	}

	t.Run("it collects every page", func(t *testing.T) {
		last := &segmentPage{
			Results:      []*management.Segment{segment("seg_3")},
			NextPageFunc: noNextPage[*string, *management.Segment, *management.ListSegmentsResponseContent](),
		}
		first := &segmentPage{
			Results:      []*management.Segment{segment("seg_1"), segment("seg_2")},
			NextPageFunc: func(context.Context) (*segmentPage, error) { return last, nil },
		}

		got, err := collectPages(context.Background(), first)

		assert.NoError(t, err)
		assert.Equal(t, []string{"seg_1", "seg_2", "seg_3"}, ids(got))
	})

	t.Run("it returns the results so far with a next-page error", func(t *testing.T) {
		first := &segmentPage{
			Results: []*management.Segment{segment("seg_1")},
			NextPageFunc: func(context.Context) (*segmentPage, error) {
				return nil, errors.New("500 Internal Server Error")
			},
		}

		got, err := collectPages(context.Background(), first)

		assert.EqualError(t, err, "500 Internal Server Error")
		assert.Equal(t, []string{"seg_1"}, ids(got))
	})
}

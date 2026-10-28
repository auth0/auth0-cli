package cli

import (
	"io"
	"testing"

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

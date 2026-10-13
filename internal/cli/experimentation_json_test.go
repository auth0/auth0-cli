package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// experimentationFixtureSchemaDoc is a minimal OpenAPI document covering the
// experimentation write operations the --data tests drive, so validation runs
// offline against a known schema.
const experimentationFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/experimentation/experiments": {
      "post": {
        "operationId": "post_experiment",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "required": ["name", "feature_flag_id", "authentication_flow"],
          "properties": {
            "name": {"type": "string"},
            "feature_flag_id": {"type": "string"},
            "authentication_flow": {"type": "string"}
          }
        }}}}
      }
    },
    "/experimentation/experiments/{id}": {
      "patch": {
        "operationId": "patch_experiment",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"name": {"type": "string"}, "status": {"type": "string"}}
        }}}}
      }
    },
    "/experimentation/feature-flags": {
      "post": {
        "operationId": "post_feature_flag",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "required": ["name", "parameters"],
          "properties": {"name": {"type": "string"}, "parameters": {"type": "object"}}
        }}}}
      }
    },
    "/experimentation/feature-flags/{id}": {
      "patch": {
        "operationId": "patch_feature_flag",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"name": {"type": "string"}, "status": {"type": "string"}}
        }}}}
      }
    },
    "/experimentation/feature-flags/{id}/variations": {
      "post": {
        "operationId": "post_variation",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "required": ["name", "overrides"],
          "properties": {"name": {"type": "string"}, "overrides": {"type": "object"}}
        }}}}
      }
    },
    "/experimentation/feature-flags/{id}/variations/{vid}": {
      "patch": {
        "operationId": "patch_variation",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"name": {"type": "string"}}
        }}}}
      }
    },
    "/experimentation/segments": {
      "post": {
        "operationId": "post_segment",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "required": ["name", "rules"],
          "properties": {"name": {"type": "string"}, "rules": {"type": "array"}}
        }}}}
      }
    },
    "/experimentation/segments/{id}": {
      "patch": {
        "operationId": "patch_segment",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"name": {"type": "string"}, "status": {"type": "string"}}
        }}}}
      }
    }
  }
}`

// useExperimentationFixtureSchema points --data validation at
// experimentationFixtureSchemaDoc for the duration of the test. It swaps the
// package-level newSchemaManager, so callers must not use t.Parallel().
func useExperimentationFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(experimentationFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

// experimentationWriteClient captures the method and URI runJSONWrite sends.
type experimentationWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

func (c *experimentationWriteClient) Request(
	_ context.Context,
	method string,
	uri string,
	_ interface{},
	_ ...management.RequestOption,
) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newExperimentationWriteCLI(client *experimentationWriteClient) *cli {
	return &cli{
		api:      &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &bytes.Buffer{}},
	}
}

func TestExperimentationWriteCmdsData(t *testing.T) {
	useExperimentationFixtureSchema(t)

	tests := []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		args       []string
		validData  string
		badData    string
		wantMethod string
		wantURI    string
	}{
		{
			name:       "experiments create",
			newCmd:     createExperimentCmd,
			validData:  `{"name":"exp","feature_flag_id":"flg_1","authentication_flow":"authentication"}`,
			badData:    `{"name":"exp"}`,
			wantMethod: http.MethodPost,
			wantURI:    "/experimentation/experiments",
		},
		{
			name:       "experiments update",
			newCmd:     updateExperimentCmd,
			args:       []string{"exp_1"},
			validData:  `{"status":"active"}`,
			badData:    `{"name":123}`,
			wantMethod: http.MethodPatch,
			wantURI:    "/experimentation/experiments/exp_1",
		},
		{
			name:       "feature-flags create",
			newCmd:     createFeatureFlagCmd,
			validData:  `{"name":"ff","parameters":{"color":{"type":"string","value":"blue"}}}`,
			badData:    `{"name":"ff"}`,
			wantMethod: http.MethodPost,
			wantURI:    "/experimentation/feature-flags",
		},
		{
			name:       "feature-flags update",
			newCmd:     updateFeatureFlagCmd,
			args:       []string{"flg_1"},
			validData:  `{"status":"active"}`,
			badData:    `{"name":123}`,
			wantMethod: http.MethodPatch,
			wantURI:    "/experimentation/feature-flags/flg_1",
		},
		{
			name:       "variations create",
			newCmd:     createVariationCmd,
			args:       []string{"flg_1"},
			validData:  `{"name":"treatment","overrides":{"color":"red"}}`,
			badData:    `{"name":"treatment"}`,
			wantMethod: http.MethodPost,
			wantURI:    "/experimentation/feature-flags/flg_1/variations",
		},
		{
			name:       "variations update",
			newCmd:     updateVariationCmd,
			args:       []string{"flg_1", "var_1"},
			validData:  `{"name":"renamed"}`,
			badData:    `{"name":123}`,
			wantMethod: http.MethodPatch,
			wantURI:    "/experimentation/feature-flags/flg_1/variations/var_1",
		},
		{
			name:       "segments create",
			newCmd:     createSegmentCmd,
			validData:  `{"name":"seg","rules":[{"match":{"domain":{"ends_with":["example.com"]}}}]}`,
			badData:    `{"name":"seg"}`,
			wantMethod: http.MethodPost,
			wantURI:    "/experimentation/segments",
		},
		{
			name:       "segments update",
			newCmd:     updateSegmentCmd,
			args:       []string{"seg_1"},
			validData:  `{"status":"active"}`,
			badData:    `{"name":123}`,
			wantMethod: http.MethodPatch,
			wantURI:    "/experimentation/segments/seg_1",
		},
	}

	for _, test := range tests {
		t.Run(test.name+" sends the validated payload", func(t *testing.T) {
			client := &experimentationWriteClient{}
			cmd := test.newCmd(newExperimentationWriteCLI(client))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, test.args...), "--data", test.validData))

			require.NoError(t, cmd.Execute())
			assert.Equal(t, test.wantMethod, client.gotMethod)
			assert.Equal(t, "https://example.test/api/v2"+test.wantURI, client.gotURI)
		})

		t.Run(test.name+" rejects an invalid payload before calling the API", func(t *testing.T) {
			client := &experimentationWriteClient{}
			cmd := test.newCmd(newExperimentationWriteCLI(client))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, test.args...), "--data", test.badData))

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "schema validation")
			assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
		})
	}
}

func TestExperimentationWriteCmdsDataExclusiveWithFieldFlags(t *testing.T) {
	cmd := createSegmentCmd(newExperimentationWriteCLI(&experimentationWriteClient{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"--data", `{"name":"seg"}`, "--name", "seg"})

	assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
}

func TestExperimentationListCmdsQuery(t *testing.T) {
	tests := []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		filterFlag []string
	}{
		{name: "experiments", newCmd: listExperimentsCmd, filterFlag: []string{"--status", "active"}},
		{name: "feature-flags", newCmd: listFeatureFlagsCmd, filterFlag: []string{"--type", "self"}},
		{name: "segments", newCmd: listSegmentsCmd, filterFlag: []string{"--type", "self"}},
	}

	for _, test := range tests {
		t.Run(test.name+" rejects an invalid --query", func(t *testing.T) {
			cli := &cli{
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
				api:      &auth0.API{},
			}
			cmd := test.newCmd(cli)
			cmd.SetArgs([]string{"--query", "not-valid-json"})

			assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
		})

		t.Run(test.name+" rejects --query combined with a filter flag", func(t *testing.T) {
			cli := &cli{
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
				api:      &auth0.API{},
			}
			cmd := test.newCmd(cli)
			cmd.SetArgs(append([]string{"--query", `{"take":1}`}, test.filterFlag...))

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
		})
	}
}

func TestExperimentationLocalErrorsAreClassified(t *testing.T) {
	tests := []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		args       []string
		wantClass  string
		wantReason string
	}{
		{
			name:       "experiments create without --feature-flag-id",
			newCmd:     createExperimentCmd,
			args:       []string{"--name", "exp"},
			wantClass:  "usage",
			wantReason: "missing_required_flags",
		},
		{
			name:       "experiments create with malformed --allocations",
			newCmd:     createExperimentCmd,
			args:       []string{"--name", "exp", "-f", "flg_1", "-a", "authentication", "-s", "percentage", "--allocations", "{oops"},
			wantClass:  "validation",
			wantReason: "malformed_json",
		},
		{
			name:       "experiments create with a decreasing --levels",
			newCmd:     createExperimentCmd,
			args:       []string{"--name", "exp", "-f", "flg_1", "-a", "authentication", "-s", "percentage", "--levels", "50,10"},
			wantClass:  "usage",
			wantReason: "invalid_flag_value",
		},
		{
			name:       "feature-flags create without --name",
			newCmd:     createFeatureFlagCmd,
			wantClass:  "usage",
			wantReason: "missing_required_flags",
		},
		{
			name:       "feature-flags create with malformed --parameters",
			newCmd:     createFeatureFlagCmd,
			args:       []string{"--name", "ff", "--parameters", "{oops"},
			wantClass:  "validation",
			wantReason: "malformed_json",
		},
		{
			name:       "segments create without --rules",
			newCmd:     createSegmentCmd,
			args:       []string{"--name", "seg"},
			wantClass:  "usage",
			wantReason: "missing_required_flags",
		},
		{
			name:       "segments create with an unknown rule attribute",
			newCmd:     createSegmentCmd,
			args:       []string{"--name", "seg", "--rules", `[{"match":{"nope":["x"]}}]`},
			wantClass:  "usage",
			wantReason: "invalid_flag_value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &experimentationWriteClient{}
			cmd := test.newCmd(newExperimentationWriteCLI(client))
			prepareInteractivity(cmd)
			cmd.SetArgs(test.args)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Equal(t, test.wantClass, errorClass(err))
			assert.Equal(t, test.wantReason, errorReason(err))
			assert.Empty(t, client.gotMethod, "must not reach the API on a local error")
		})
	}
}

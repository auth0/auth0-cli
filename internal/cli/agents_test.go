package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	management1 "github.com/auth0/go-auth0/management"
	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func agentPage(agents ...*management.AgentResponseContent) *auth0.AgentListPage {
	return &auth0.AgentListPage{
		Results:      agents,
		NextPageFunc: noNextPage[*string, *management.AgentResponseContent, *management.ListAgentsResponseContent](),
	}
}

func newAgentsTestCLI(agentsAPI auth0.AgentsAPI, stdout io.Writer) *cli {
	return &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
		apiv3: &auth0.APIV3{Agents: agentsAPI},
	}
}

func TestParseAgentMetadata(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]any
		wantNil bool
		wantErr bool
	}{
		{
			name: "object keeps value types",
			raw:  `{"team":"ai","n":1}`,
			want: map[string]any{"team": "ai", "n": float64(1)},
		},
		{
			name:    "null yields a nil map",
			raw:     `null`,
			wantNil: true,
		},
		{
			name: "empty object yields an empty map",
			raw:  `{}`,
			want: map[string]any{},
		},
		{name: "array is rejected", raw: `[1]`, wantErr: true},
		{name: "string is rejected", raw: `"x"`, wantErr: true},
		{name: "invalid json is rejected", raw: `not-json`, wantErr: true},
		{name: "empty string is rejected", raw: ``, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAgentMetadata(tt.raw)

			if tt.wantErr {
				assert.ErrorContains(t, err, "--metadata")
				var vErr validationError
				assert.True(t, errors.As(err, &vErr))
				return
			}

			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAgentsListCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		format        display.OutputFormat
		expectedTake  int
		expectedError string
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name:         "it lists agents with the default page size",
			expectedTake: 100,
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "agt_001")
				assert.Contains(t, out, "support-bot")
				assert.Contains(t, out, "ext-1")
			},
		},
		{
			name:         "it caps take at --number",
			args:         []string{"--number", "5"},
			expectedTake: 5,
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "support-bot")
			},
		},
		{
			name:         "it renders the raw agent object as json",
			format:       display.OutputFormatJSON,
			expectedTake: 100,
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, `"agent_id": "agt_001"`)
			},
		},
		{
			name:          "it rejects an out of range --number without calling the API",
			args:          []string{"--number", "0"},
			expectedError: "number flag invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			if test.expectedError == "" {
				agentsAPI.EXPECT().
					List(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *management.ListAgentsRequestParameters, _ ...option.RequestOption) (*auth0.AgentListPage, error) {
						if assert.NotNil(t, req) && assert.NotNil(t, req.Take) {
							assert.Equal(t, test.expectedTake, *req.Take)
						}
						return agentPage(&management.AgentResponseContent{
							AgentID:         "agt_001",
							Name:            "support-bot",
							ExternalAgentID: auth0.String("ext-1"),
						}), nil
					})
			}

			stdout := &bytes.Buffer{}
			cli := newAgentsTestCLI(agentsAPI, stdout)
			cli.renderer.Format = test.format

			cmd := listAgentsCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			test.assertOutput(t, stdout.String())
		})
	}
}

func TestAgentsShowCmd(t *testing.T) {
	tests := []struct {
		name          string
		apiError      error
		expectedError string
	}{
		{name: "it shows the agent with its metadata"},
		{
			name:          "it returns an error if the API call fails",
			apiError:      errors.New("404 Not Found: inexistent_agent"),
			expectedError: `failed to get agent "agt_001": 404 Not Found: inexistent_agent`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			agentsAPI.EXPECT().
				Read(gomock.Any(), "agt_001").
				Return(&management.AgentResponseContent{
					AgentID:  "agt_001",
					Name:     "support-bot",
					Metadata: management.AgentMetadata{"team": "ai"},
				}, test.apiError)

			stdout := &bytes.Buffer{}
			cmd := showAgentCmd(newAgentsTestCLI(agentsAPI, stdout))
			cmd.SetArgs([]string{"agt_001"})
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Contains(t, stdout.String(), "support-bot")
			assert.Contains(t, stdout.String(), `{"team":"ai"}`)
		})
	}
}

func TestAgentsCreateCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		apiError      error
		expectCreate  bool
		expectedError string
		assertRequest func(t testing.TB, req *management.CreateAgentRequestContent)
	}{
		{
			name:         "it sends only the name when no optional flags are set",
			args:         []string{"--name", "bot"},
			expectCreate: true,
			assertRequest: func(t testing.TB, req *management.CreateAgentRequestContent) {
				assert.Equal(t, "bot", req.Name)
				assert.Nil(t, req.ClientID)
				assert.Nil(t, req.ExternalAgentID)
				assert.Nil(t, req.Metadata)
			},
		},
		{
			name: "it sends every optional field when set",
			args: []string{
				"--name", "bot",
				"--client-id", "cli_1",
				"--external-id", "ext-1",
				"--metadata", `{"team":"ai"}`,
			},
			expectCreate: true,
			assertRequest: func(t testing.TB, req *management.CreateAgentRequestContent) {
				assert.Equal(t, auth0.String("cli_1"), req.ClientID)
				assert.Equal(t, auth0.String("ext-1"), req.ExternalAgentID)
				if assert.NotNil(t, req.Metadata) {
					assert.Equal(t, management.AgentMetadata{"team": "ai"}, *req.Metadata)
				}
			},
		},
		{
			name:          "it rejects invalid --metadata before calling the API",
			args:          []string{"--name", "bot", "--metadata", "[1]"},
			expectedError: "invalid JSON for --metadata",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{"--name", "bot"},
			apiError:      errors.New("409 Conflict: agent_conflict"),
			expectCreate:  true,
			expectedError: `failed to create agent "bot": 409 Conflict: agent_conflict`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			if test.expectCreate {
				agentsAPI.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *management.CreateAgentRequestContent, _ ...option.RequestOption) (*management.AgentResponseContent, error) {
						if test.assertRequest != nil {
							test.assertRequest(t, req)
						}
						if test.apiError != nil {
							return nil, test.apiError
						}
						return &management.AgentResponseContent{AgentID: "agt_001", Name: req.Name}, nil
					})
			}

			cmd := createAgentCmd(newAgentsTestCLI(agentsAPI, io.Discard))
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAgentsUpdateCmd(t *testing.T) {
	const agtID = "agt_001"

	tests := []struct {
		name          string
		args          []string
		apiError      error
		expectUpdate  bool
		expectedBody  string
		expectedError string
	}{
		{
			name:         "it updates the name",
			args:         []string{agtID, "--name", "new"},
			expectUpdate: true,
			expectedBody: `{"name":"new"}`,
		},
		{
			name:          "it returns an error when nothing changes",
			args:          []string{agtID, "--name", "old"},
			expectedError: "nothing to update: provide at least one flag or --data",
		},
		{
			name:         "it sends the metadata object",
			args:         []string{agtID, "--metadata", `{"a":1}`},
			expectUpdate: true,
			expectedBody: `{"metadata":{"a":1}}`,
		},
		{
			name:         "it sends an explicit null to clear metadata",
			args:         []string{agtID, "--metadata", "null"},
			expectUpdate: true,
			expectedBody: `{"metadata":null}`,
		},
		{
			name:         "it sends an explicit empty metadata object",
			args:         []string{agtID, "--metadata", "{}"},
			expectUpdate: true,
			expectedBody: `{"metadata":{}}`,
		},
		{
			name:          "it rejects invalid --metadata",
			args:          []string{agtID, "--metadata", "bad"},
			expectedError: "invalid JSON for --metadata",
		},
		{
			name:          "it returns an error if the API call fails",
			args:          []string{agtID, "--name", "new"},
			apiError:      errors.New("500 Internal Server Error"),
			expectUpdate:  true,
			expectedError: `failed to update agent "agt_001": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			agentsAPI.EXPECT().
				Read(gomock.Any(), agtID).
				Return(&management.AgentResponseContent{AgentID: agtID, Name: "old"}, nil)
			if test.expectUpdate {
				agentsAPI.EXPECT().
					Update(gomock.Any(), agtID, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, req *management.PatchAgentRequestParameters, _ ...option.RequestOption) (*management.AgentResponseContent, error) {
						if test.expectedBody != "" {
							body, err := json.Marshal(req)
							assert.NoError(t, err)
							assert.JSONEq(t, test.expectedBody, string(body))
						}
						if test.apiError != nil {
							return nil, test.apiError
						}
						return &management.AgentResponseContent{AgentID: agtID, Name: "new"}, nil
					})
			}

			cmd := updateAgentCmd(newAgentsTestCLI(agentsAPI, io.Discard))
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAgentsDeleteCmd(t *testing.T) {
	tests := []struct {
		name          string
		ids           []string
		readErr       error
		deleteErr     error
		expectDelete  bool
		expectedError string
	}{
		{
			name:         "it deletes a single agent",
			ids:          []string{"agt_001"},
			expectDelete: true,
		},
		{
			name:         "it deletes multiple agents",
			ids:          []string{"agt_001", "agt_002"},
			expectDelete: true,
		},
		{
			// Delete returns 204 for an unknown ID, so the 404 comes from the lookup.
			name:          "it returns the not-found error for an unknown agent",
			ids:           []string{"agt_404"},
			readErr:       errors.New("404 Not Found: inexistent_agent"),
			expectedError: `failed to delete agent "agt_404": 404 Not Found: inexistent_agent`,
		},
		{
			name:          "it returns an error if the delete call fails",
			ids:           []string{"agt_001"},
			deleteErr:     errors.New("500 Internal Server Error"),
			expectDelete:  true,
			expectedError: `failed to delete agent "agt_001": 500 Internal Server Error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			for _, id := range test.ids {
				read := agentsAPI.EXPECT().Read(gomock.Any(), id).Return(&management.AgentResponseContent{AgentID: id}, test.readErr)
				if test.expectDelete {
					agentsAPI.EXPECT().Delete(gomock.Any(), id).Return(test.deleteErr).After(read)
				}
			}

			cmd := deleteAgentCmd(newAgentsTestCLI(agentsAPI, io.Discard))
			cmd.SetArgs(append(test.ids, "--force"))
			err := cmd.Execute()

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAgentsDeleteCmdRequiresForceInAgentMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cli := newAgentsTestCLI(mock.NewMockAgentsAPI(ctrl), io.Discard)
	cli.agentMode = true

	cmd := deleteAgentCmd(cli)
	cmd.SetArgs([]string{"agt_001"})

	assert.ErrorIs(t, cmd.Execute(), errDestructiveNoConfirm)
}

func TestAgentPickerOptions(t *testing.T) {
	tests := []struct {
		name          string
		agents        []*management.AgentResponseContent
		expectedValue []string
		expectedError string
	}{
		{
			name: "it returns one option per agent",
			agents: []*management.AgentResponseContent{
				{AgentID: "agt_001", Name: "support-bot"},
				{AgentID: "agt_002", Name: "billing-bot"},
			},
			expectedValue: []string{"agt_001", "agt_002"},
		},
		{
			name:          "it returns an error when there are no agents",
			expectedError: "no agents available. Create one by running: `auth0 agents create`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			agentsAPI := mock.NewMockAgentsAPI(ctrl)
			agentsAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(agentPage(test.agents...), nil)

			cli := newAgentsTestCLI(agentsAPI, io.Discard)
			options, err := cli.agentPickerOptions(context.Background())

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			var values []string
			for _, o := range options {
				values = append(values, o.value)
			}
			assert.Equal(t, test.expectedValue, values)
			assert.Contains(t, options[0].label, "support-bot")
		})
	}
}

func TestAgentCreateShowsNextSteps(t *testing.T) {
	messages := &bytes.Buffer{}
	r := &display.Renderer{MessageWriter: messages, ResultWriter: io.Discard}

	assert.NoError(t, r.AgentCreate(&management.AgentResponseContent{AgentID: "agt_123", Name: "bot"}))

	for _, want := range []string{
		"auth0 agents show agt_123",
		"auth0 agents update agt_123",
		"auth0 apps update <app-id> --agent-id agt_123",
		"auth0 apps update <app-id> --unlink-agent",
	} {
		assert.Contains(t, messages.String(), want)
	}
}

func TestAgentClientPickerOptions(t *testing.T) {
	tests := []struct {
		name       string
		clients    []*management1.Client
		listErr    error
		wantValues []string
		wantErr    string
	}{
		{
			name:       "none option leads the m2m apps",
			clients:    []*management1.Client{{ClientID: auth0.String("c1"), Name: auth0.String("Bot App")}},
			wantValues: []string{"", "c1"},
		},
		{
			name:       "no apps still offers none",
			wantValues: []string{""},
		},
		{
			name:    "list error is returned",
			listErr: errors.New("boom"),
			wantErr: "failed to list applications: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(authModeEnvVar, authModeEnv)
			ctrl := gomock.NewController(t)

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&management1.ClientList{Clients: test.clients}, test.listErr)

			c := &cli{api: &auth0.API{Client: clientAPI}}
			opts, err := c.agentClientPickerOptions(context.Background())

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}
			assert.NoError(t, err)
			var values []string
			for _, o := range opts {
				values = append(values, o.value)
			}
			assert.Equal(t, test.wantValues, values)
			assert.Equal(t, "None (don't link an application)", opts[0].label)
		})
	}
}

func TestParseAgentMetadataFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	assert.NoError(t, os.WriteFile(path, []byte(`{"team":"ai"}`), 0600))

	got, err := parseAgentMetadata("@" + path)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"team": "ai"}, got)

	_, err = parseAgentMetadata("@" + filepath.Join(t.TempDir(), "missing.json"))
	assert.ErrorContains(t, err, "failed to read --metadata")
}

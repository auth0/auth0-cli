package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestAppsListCmd(t *testing.T) {
	tests := []struct {
		name         string
		assertOutput func(t testing.TB, out string)
		args         []string
	}{
		{
			name: "happy path",
			assertOutput: func(t testing.TB, out string) {
				expectTable(t, out,
					[]string{"CLIENT ID", "NAME", "TYPE", "RESOURCE SERVER"},
					[][]string{
						{"some-id", "some-name", "Generic", ""},
					},
				)
			},
		},
		{
			name: "filter by agent id",
			args: []string{"--agent-id", "agt_001"},
			assertOutput: func(t testing.TB, out string) {
				expectTable(t, out,
					[]string{"CLIENT ID", "NAME", "TYPE", "RESOURCE SERVER"},
					[][]string{
						{"some-id", "some-name", "Generic", ""},
					},
				)
			},
		},
		{
			name: "reveal secrets",
			args: []string{"--reveal-secrets"},
			assertOutput: func(t testing.TB, out string) {
				expectTable(t, out,
					[]string{"CLIENT ID", "NAME", "TYPE", "CLIENT SECRET", "RESOURCE SERVER"},
					[][]string{
						{"some-id", "some-name", "Generic", "secret-here", ""},
					},
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Step 1: Setup our client mock for this test. We only care about
			// Clients so no need to bootstrap other bits.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&management.ClientList{
					Clients: []*management.Client{
						{
							Name:         auth0.String("some-name"),
							ClientID:     auth0.String("some-id"),
							Callbacks:    &[]string{"http://localhost"},
							ClientSecret: auth0.String("secret-here"),
						},
					},
				}, nil)

			stdout := &bytes.Buffer{}

			// Step 2: Setup our cli context. The important bits are
			// renderer and api.
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				api: &auth0.API{Client: clientAPI},
			}

			cmd := listAppsCmd(cli)
			cmd.SetArgs(test.args)

			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}

			test.assertOutput(t, stdout.String())
		})
	}
}

func TestAppsCreateCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name: "Resource Server - resource-server-identifier is empty string",
			args: []string{
				"--name", "My Resource Server App",
				"--type", "resource_server",
				"--resource-server-identifier", "",
			},
			expectedError: "resource-server-identifier cannot be empty for resource_server app type",
		},
		{
			name: "Resource Server - resource-server-identifier is whitespace only",
			args: []string{
				"--name", "My Resource Server App",
				"--type", "resource_server",
				"--resource-server-identifier", "   ",
			},
			expectedError: "resource-server-identifier cannot be empty for resource_server app type",
		},
		{
			name: "Resource Server - resource-server-identifier is tab/newline",
			args: []string{
				"--name", "My Resource Server App",
				"--type", "resource_server",
				"--resource-server-identifier", "\t\n",
			},
			expectedError: "resource-server-identifier cannot be empty for resource_server app type",
		},
		{
			name: "Organization - discovery methods without require behavior",
			args: []string{
				"--name", "My App",
				"--type", "regular",
				"--organization-usage", "require",
				"--organization-discovery-methods", "email",
			},
			expectedError: "--organization-discovery-methods requires --organization-require-behavior=pre_login_prompt",
		},
		{
			name: "Organization - discovery methods with wrong require behavior",
			args: []string{
				"--name", "My App",
				"--type", "regular",
				"--organization-usage", "require",
				"--organization-require-behavior", "no_prompt",
				"--organization-discovery-methods", "email",
			},
			expectedError: `--organization-discovery-methods requires --organization-require-behavior=pre_login_prompt, but got "no_prompt"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cli := &cli{}
			cli.noInput = true // Non-interactive mode.
			cmd := createAppCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			assert.EqualError(t, err, test.expectedError)
		})
	}
}

// TestAppsCreateCmdSucceedsWithoutSavedConfig guards the fix for a create that
// touches the API before persisting the default app locally: with no saved
// config (an empty HOME, as in a read-only sandbox), the app is still created
// via the API and the command must succeed, treating the failed local
// default-app write as a non-fatal warning rather than reporting failure for an
// app that already exists.
func TestAppsListCmdAgentIDExclusiveWithQuery(t *testing.T) {
	cmd := listAppsCmd(&cli{})
	cmd.SetArgs([]string{"--agent-id", "agt_001", "--query", "{}"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "query")
	assert.ErrorContains(t, err, "agent-id")
}

func TestAppsCreateCmdAgentID(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		assertClient func(t testing.TB, c *management.Client)
	}{
		{
			name: "links the app to the agent when --agent-id is set",
			args: []string{"--name", "x", "--type", "m2m", "--agent-id", "agt_001"},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Equal(t, "agt_001", c.GetAgentID())
			},
		},
		{
			name: "leaves agent_id unset when --agent-id is omitted",
			args: []string{"--name", "x", "--type", "m2m"},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Nil(t, c.AgentID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, c *management.Client, _ ...management.RequestOption) error {
					test.assertClient(t, c)
					c.ClientID = auth0.String("created-client-id")
					return nil
				})

			cli := &cli{
				noInput: true,
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				api: &auth0.API{Client: clientAPI},
			}

			cmd := createAppCmd(cli)
			cmd.SetArgs(test.args)

			assert.NoError(t, cmd.Execute())
		})
	}
}

func TestAppsCreateCmdSucceedsWithoutSavedConfig(t *testing.T) {
	// Point config resolution at an empty temp home so no real config is read or
	// written and SetDefaultAppIDForTenant fails the way it would in a sandbox.
	t.Setenv("HOME", t.TempDir())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientAPI := mock.NewMockClientAPI(ctrl)
	clientAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c *management.Client, _ ...management.RequestOption) error {
			c.ClientID = auth0.String("created-client-id")
			return nil
		})

	cli := &cli{
		noInput: true,
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		api: &auth0.API{Client: clientAPI},
	}

	cmd := createAppCmd(cli)
	cmd.SetArgs([]string{"--name", "My App", "--type", "regular"})

	assert.NoError(t, cmd.Execute())
}

// TestAppsUseCmdRejectedInEnvAuthMode guards that 'apps use', whose only job is
// to persist a default application into the on-disk config, fails fast with a
// clear, tagged error in env auth mode (which does not use that config) instead
// of surfacing a confusing "config file is missing" error from the write.
func TestAppsUseCmdRejectedInEnvAuthMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(authModeEnvVar, authModeEnv)

	cli := &cli{
		noInput: true,
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
	}

	cmd := useAppCmd(cli)
	cmd.SetArgs([]string{"--none"})

	err := cmd.Execute()
	assert.Error(t, err)

	var authErr authError
	assert.True(t, errors.As(err, &authErr))
	assert.Equal(t, "env_auth_not_persistable", authErr.reason)
}

func TestAppsUpdateCmdOrganizationFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		assertClient func(t testing.TB, c *management.Client)
	}{
		{
			name: "sets all organization fields when flags are provided",
			args: []string{
				"some-id",
				"--organization-usage", "require",
				"--organization-require-behavior", "pre_login_prompt",
				"--organization-discovery-methods", "email,organization_name",
			},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Equal(t, "require", c.GetOrganizationUsage())
				assert.Equal(t, "pre_login_prompt", c.GetOrganizationRequireBehavior())
				assert.Equal(t, []string{"email", "organization_name"}, c.GetOrganizationDiscoveryMethods())
			},
		},
		{
			name: "leaves organization fields unset when flags are omitted",
			args: []string{"some-id"},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Nil(t, c.OrganizationUsage)
				assert.Nil(t, c.OrganizationRequireBehavior)
				assert.Nil(t, c.OrganizationDiscoveryMethods)
			},
		},
		{
			name: "updates only discovery methods when require behavior is already set server-side",
			args: []string{
				"some-id",
				"--organization-discovery-methods", "email",
			},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Nil(t, c.OrganizationRequireBehavior)
				assert.Equal(t, []string{"email"}, c.GetOrganizationDiscoveryMethods())
			},
		},
		{
			name: "strips empty discovery method entries from a trailing comma",
			args: []string{
				"some-id",
				"--organization-require-behavior", "pre_login_prompt",
				"--organization-discovery-methods", "email,",
			},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Equal(t, []string{"email"}, c.GetOrganizationDiscoveryMethods())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Read(gomock.Any(), "some-id", gomock.Any()).
				Return(&management.Client{
					Name:    auth0.String("some-name"),
					AppType: auth0.String("regular_web"),
				}, nil)

			var captured *management.Client
			clientAPI.EXPECT().
				Update(gomock.Any(), "some-id", gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, c *management.Client, _ ...management.RequestOption) error {
					captured = c
					return nil
				})

			cli := &cli{
				noInput: true,
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  io.Discard,
				},
				api: &auth0.API{Client: clientAPI},
			}

			cmd := updateAppCmd(cli)
			cmd.SetArgs(test.args)

			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}

			test.assertClient(t, captured)
		})
	}
}

func TestAppsUpdateCmdAgentFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		returnAgentID *string
		assertClient  func(t testing.TB, c *management.Client)
		assertOutput  func(t testing.TB, out string)
	}{
		{
			name:          "links the app to the agent when --agent-id is set",
			args:          []string{"some-id", "--agent-id", "agt_001"},
			returnAgentID: auth0.String("agt_001"),
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Equal(t, "agt_001", c.GetAgentID())
			},
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "AGENT ID")
				assert.Contains(t, out, "agt_001")
			},
		},
		{
			name: "leaves agent_id unset when no agent flag is set",
			args: []string{"some-id"},
			assertClient: func(t testing.TB, c *management.Client) {
				assert.Nil(t, c.AgentID)
			},
			assertOutput: func(t testing.TB, out string) {
				assert.NotContains(t, out, "AGENT ID")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Read(gomock.Any(), "some-id", gomock.Any()).
				Return(&management.Client{Name: auth0.String("some-name"), AppType: auth0.String("non_interactive")}, nil)
			clientAPI.EXPECT().
				Update(gomock.Any(), "some-id", gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, c *management.Client, _ ...management.RequestOption) error {
					test.assertClient(t, c)
					c.AgentID = test.returnAgentID
					return nil
				})

			stdout := &bytes.Buffer{}
			cli := &cli{
				noInput:  true,
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: stdout},
				api:      &auth0.API{Client: clientAPI},
			}

			cmd := updateAppCmd(cli)
			cmd.SetArgs(test.args)
			assert.NoError(t, cmd.Execute())
			test.assertOutput(t, stdout.String())
		})
	}
}

func TestAppsUpdateCmdUnlinkAgent(t *testing.T) {
	tests := []struct {
		name      string
		updateErr error
		wantErr   string
	}{
		{name: "unlinks in a single update and shows the app read back"},
		{
			name:      "update failure leaves nothing half-applied",
			updateErr: errors.New("boom"),
			wantErr:   `failed to update application with ID "some-id": boom`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			existing := &management.Client{Name: auth0.String("some-name"), AppType: auth0.String("non_interactive"), AgentID: auth0.String("agt_001")}
			read := clientAPI.EXPECT().Read(gomock.Any(), "some-id", gomock.Any()).Return(existing, nil)
			update := clientAPI.EXPECT().
				Update(gomock.Any(), "some-id", nil, gomock.Any()).
				After(read).
				Return(test.updateErr)
			if test.updateErr == nil {
				clientAPI.EXPECT().
					Read(gomock.Any(), "some-id", gomock.Any()).
					After(update).
					Return(&management.Client{Name: auth0.String("some-name"), AppType: auth0.String("non_interactive")}, nil)
			}

			stdout := &bytes.Buffer{}
			cli := &cli{
				noInput:  true,
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: stdout},
				api:      &auth0.API{Client: clientAPI},
			}

			cmd := updateAppCmd(cli)
			cmd.SetArgs([]string{"some-id", "--unlink-agent"})
			err := cmd.Execute()

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Contains(t, stdout.String(), "some-name")
			assert.NotContains(t, stdout.String(), "AGENT ID")
		})
	}
}

func TestUnlinkAgentPatchBody(t *testing.T) {
	body, err := unlinkAgentPatchBody(&management.Client{Name: auth0.String("renamed")})
	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, map[string]interface{}{"name": "renamed", "agent_id": nil}, got)
}

func TestAppsUpdateCmdAgentFlagsMutuallyExclusive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cli := &cli{
		noInput: true,
		api:     &auth0.API{Client: mock.NewMockClientAPI(ctrl)},
	}

	cmd := updateAppCmd(cli)
	cmd.SetArgs([]string{"some-id", "--agent-id", "agt_001", "--unlink-agent"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "agent-id")
	assert.ErrorContains(t, err, "unlink-agent")
}

func TestAppsShowCmdAgentID(t *testing.T) {
	tests := []struct {
		name         string
		agentID      *string
		assertOutput func(t testing.TB, out string)
	}{
		{
			name:    "renders the agent id when the app is linked",
			agentID: auth0.String("agt_001"),
			assertOutput: func(t testing.TB, out string) {
				assert.Contains(t, out, "AGENT ID")
				assert.Contains(t, out, "agt_001")
			},
		},
		{
			name: "omits the agent id row when the app is not linked",
			assertOutput: func(t testing.TB, out string) {
				assert.NotContains(t, out, "AGENT ID")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				Read(gomock.Any(), "some-id", gomock.Any()).
				Return(&management.Client{
					ClientID: auth0.String("some-id"),
					Name:     auth0.String("some-name"),
					AppType:  auth0.String("non_interactive"),
					AgentID:  test.agentID,
				}, nil)

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  stdout,
				},
				api: &auth0.API{Client: clientAPI},
			}

			cmd := showAppCmd(cli)
			cmd.SetArgs([]string{"some-id"})

			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			test.assertOutput(t, stdout.String())
		})
	}
}

func TestFormatAppSettingsPath(t *testing.T) {
	assert.Empty(t, formatAppSettingsPath(""))
	assert.Equal(t, "applications/app-id-1/settings", formatAppSettingsPath("app-id-1"))
}

func TestTypeFor(t *testing.T) {
	testAppType := appTypeNative
	expected := "Native"
	assert.Equal(t, &expected, typeFor(&testAppType))

	testAppType = appTypeSPA
	expected = "Single Page Web Application"
	assert.Equal(t, &expected, typeFor(&testAppType))

	testAppType = appTypeRegularWeb
	expected = "Regular Web Application"
	assert.Equal(t, &expected, typeFor(&testAppType))

	testAppType = appTypeNonInteractive
	expected = "Machine to Machine"
	assert.Equal(t, &expected, typeFor(&testAppType))

	testAppType = appTypeResourceServer
	expected = "Resource Server"
	assert.Equal(t, &expected, typeFor(&testAppType))

	testAppType = "some-unknown-api-type"
	assert.Nil(t, typeFor(&testAppType))
}

func TestCommaSeparatedStringToSlice(t *testing.T) {
	assert.Equal(t, []string{}, commaSeparatedStringToSlice(""))
	assert.Equal(t, []string{"foo", "bar", "baz"}, commaSeparatedStringToSlice(" foo  , bar , baz "))
}

func TestFilterEmptyStrings(t *testing.T) {
	assert.Equal(t, []string{}, excludeEmptyEntries([]string{}))
	assert.Equal(t, []string{}, excludeEmptyEntries([]string{""}))
	assert.Equal(t, []string{}, excludeEmptyEntries([]string{"", ""}))
	assert.Equal(t, []string{"a"}, excludeEmptyEntries([]string{"a"}))
	assert.Equal(t, []string{"a", "b"}, excludeEmptyEntries([]string{"a", "", "b"}))
}

// newClientSearchPages chains pages of search results, ending with core.ErrNoPages.
func newClientSearchPages(pages ...[]*managementv3.ClientSearchResponse) *auth0.ClientSearchPage {
	var build func(i int) *auth0.ClientSearchPage
	build = func(i int) *auth0.ClientSearchPage {
		return &auth0.ClientSearchPage{
			Results: pages[i],
			NextPageFunc: func(context.Context) (*auth0.ClientSearchPage, error) {
				if i+1 >= len(pages) {
					return nil, core.ErrNoPages
				}
				return build(i + 1), nil
			},
		}
	}

	return build(0)
}

// searchClients returns one search result per client ID.
func searchClients(ids ...string) []*managementv3.ClientSearchResponse {
	clients := make([]*managementv3.ClientSearchResponse, 0, len(ids))
	for _, id := range ids {
		clients = append(clients, &managementv3.ClientSearchResponse{ClientID: auth0.String(id)})
	}

	return clients
}

func TestClientSortField(t *testing.T) {
	tests := []struct {
		input   string
		want    managementv3.ClientSortFieldEnum
		wantErr string
	}{
		{input: "name", want: managementv3.ClientSortFieldEnumName},
		{input: "updated_at", want: managementv3.ClientSortFieldEnumUpdatedAt},
		{input: "name:1", want: managementv3.ClientSortFieldEnumName},
		{input: "name:-1", wantErr: `invalid --sort "name:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`},
		{input: "app_type", wantErr: `invalid --sort "app_type": must be one of name,updated_at`},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := clientSortField(test.input)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBuildClientSearchRequest(t *testing.T) {
	tests := []struct {
		name    string
		inputs  searchInputs
		want    *managementv3.SearchClientsRequestParameters
		wantErr string
	}{
		{
			name:   "defaults to lucene and omits unset params",
			inputs: searchInputs{Number: 50},
			want: &managementv3.SearchClientsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "maps every flag onto the request",
			inputs: searchInputs{Filter: `name co "billing"`, Parser: "scim", Sort: "updated_at:1", Fields: "client_id,name", Number: 20},
			want: &managementv3.SearchClientsRequestParameters{
				Q:      auth0.String(`name co "billing"`),
				Parser: managementv3.SearchParserEnumSCIM.Ptr(),
				Sort:   managementv3.ClientSortFieldEnumUpdatedAt.Ptr(),
				Fields: auth0.String("client_id,name"),
				Take:   auth0.Int(20),
			},
		},
		{
			name:   "sends include_fields=false with --exclude-fields",
			inputs: searchInputs{Fields: "callbacks", ExcludeFields: true, Number: 50},
			want: &managementv3.SearchClientsRequestParameters{
				Fields:        auth0.String("callbacks"),
				IncludeFields: auth0.Bool(false),
				Parser:        managementv3.SearchParserEnumLucene.Ptr(),
				Take:          auth0.Int(50),
			},
		},
		{
			name:   "caps the page size at 100",
			inputs: searchInputs{Number: 1000},
			want: &managementv3.SearchClientsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(100),
			},
		},
		{
			name:    "rejects an invalid sort",
			inputs:  searchInputs{Sort: "app_type", Number: 50},
			wantErr: `invalid --sort "app_type": must be one of name,updated_at`,
		},
		{
			name:    "rejects invalid shared inputs",
			inputs:  searchInputs{ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildClientSearchRequest(test.inputs)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSearchAppsCmd(t *testing.T) {
	thousandIDs := make([]string, 1000)
	for i := range thousandIDs {
		thousandIDs[i] = fmt.Sprintf("client_%d", i)
	}

	tests := []struct {
		name          string
		args          []string
		searchPage    *auth0.ClientSearchPage
		searchErr     error
		serverStatus  int
		serverBody    string
		assertRequest func(t testing.TB, req *managementv3.SearchClientsRequestParameters)
		assertHTTP    func(t testing.TB, r *http.Request)
		assertOutput  func(t testing.TB, stdout, stderr string)
		wantErr       string
	}{
		{
			name:       "sends the flags to the search endpoint",
			args:       []string{"--filter", `name:"My App"`, "-s", "name", "--fields", "client_id,name", "-n", "10"},
			searchPage: newClientSearchPages(searchClients("client_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientsRequestParameters) {
				assert.Equal(t, `name:"My App"`, *req.Q)
				assert.Equal(t, managementv3.SearchParserEnumLucene, *req.Parser)
				assert.Equal(t, managementv3.ClientSortFieldEnumName, *req.Sort)
				assert.Equal(t, "client_id,name", *req.Fields)
				assert.Equal(t, 10, *req.Take)
			},
		},
		{
			name: "renders the apps list columns",
			searchPage: newClientSearchPages([]*managementv3.ClientSearchResponse{{
				ClientID:                 auth0.String("client_1"),
				Name:                     auth0.String("My App"),
				AppType:                  managementv3.ClientAppTypeEnumSpa.Ptr(),
				ResourceServerIdentifier: auth0.String("https://api.example.com"),
			}}),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				expectTable(t, stdout,
					[]string{"CLIENT ID", "NAME", "TYPE", "RESOURCE SERVER"},
					[][]string{{"client_1", "My App", "Single Page Web Application", "https://api.example.com"}},
				)
			},
		},
		{
			name:       "sends --exclude-fields as include_fields=false",
			args:       []string{"--fields", "app_type", "--exclude-fields"},
			searchPage: newClientSearchPages(searchClients("client_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientsRequestParameters) {
				assert.Equal(t, "app_type", *req.Fields)
				assert.False(t, *req.IncludeFields)
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "CLIENT ID")
				assert.NotContains(t, stdout, "TYPE")
			},
		},
		{
			name:       "shows JSON when --fields leaves no table column",
			args:       []string{"--fields", "description"},
			searchPage: newClientSearchPages(searchClients("client_1")),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "No table columns (client_id, name, app_type, resource_server_identifier) are left in the response; showing JSON instead.")
				assert.Contains(t, stdout, `"client_id": "client_1"`)
			},
		},
		{
			name:       "collects results across pages up to --number",
			args:       []string{"--filter", "name:*", "-n", "3", "--json"},
			searchPage: newClientSearchPages(searchClients("client_1", "client_2"), searchClients("client_3", "client_4")),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "client_3")
				assert.NotContains(t, stdout, "client_4")
				assert.Contains(t, stderr, "Results may be limited by --number; more may match.")
			},
		},
		{
			name:       "does not suggest raising --number when it is already at the maximum",
			args:       []string{"-n", "1000", "--json"},
			searchPage: newClientSearchPages(searchClients(thousandIDs...)),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "more may match. Refine --filter.")
				assert.NotContains(t, stderr, "raise --number")
			},
		},
		{
			name:       "renders an empty result",
			args:       []string{"--filter", `name:"missing"`},
			searchPage: newClientSearchPages(searchClients()),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "applications (0)")
				assert.Contains(t, stderr, "Try a broader --filter")
			},
		},
		{
			name:       "sends --query JSON params to the search endpoint",
			args:       []string{"-q", `{"q":"name co \"billing\"","parser":"scim"}`},
			serverBody: `{"clients":[{"client_id":"client_1"}]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Equal(t, "/api/v2/clients/search", r.URL.Path)
				assert.Equal(t, `name co "billing"`, r.URL.Query().Get("q"))
				assert.Equal(t, "scim", r.URL.Query().Get("parser"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "client_1")
			},
		},
		{
			name:       "allows --number with --query, which ignores it",
			args:       []string{"-q", `{"q":"name:*"}`, "-n", "5"},
			serverBody: `{"clients":[]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Empty(t, r.URL.Query().Get("take"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, `--number is ignored with --query; set "take" (max 100) in the JSON instead.`)
			},
		},
		{
			name:    "rejects --query combined with --filter",
			args:    []string{"-q", `{"q":"name:*"}`, "--filter", "name:*"},
			wantErr: "if any flags in the group [query filter] are set none of the others can be; [filter query] were all set",
		},
		{
			name:    "rejects --query combined with --parser",
			args:    []string{"-q", `{"q":"name:*"}`, "--parser", "scim"},
			wantErr: "if any flags in the group [query parser] are set none of the others can be; [parser query] were all set",
		},
		{
			name:    "rejects --query combined with --sort",
			args:    []string{"-q", `{"q":"name:*"}`, "--sort", "name"},
			wantErr: "if any flags in the group [query sort] are set none of the others can be; [query sort] were all set",
		},
		{
			name:    "rejects --query combined with --fields",
			args:    []string{"-q", `{"q":"name:*"}`, "--fields", "name"},
			wantErr: "if any flags in the group [query fields] are set none of the others can be; [fields query] were all set",
		},
		{
			name:    "rejects --query combined with --exclude-fields",
			args:    []string{"-q", `{"q":"name:*"}`, "--exclude-fields"},
			wantErr: "if any flags in the group [query exclude-fields] are set none of the others can be; [exclude-fields query] were all set",
		},
		{
			name:    "rejects --csv when --fields leaves no table column",
			args:    []string{"--fields", "description", "--csv"},
			wantErr: "--csv needs at least one table column (client_id, name, app_type, resource_server_identifier) left by --fields",
		},
		{
			name:    "rejects an invalid --sort before calling the API",
			args:    []string{"-s", "app_type"},
			wantErr: `invalid --sort "app_type": must be one of name,updated_at`,
		},
		{
			name:    "rejects an out-of-range --number before calling the API",
			args:    []string{"--filter", "name:*", "-n", "1001"},
			wantErr: "number flag invalid, please pass a number between 1 and 1000",
		},
		{
			name:    "does not offer --reveal-secrets, as search never returns secrets",
			args:    []string{"--reveal-secrets"},
			wantErr: "unknown flag: --reveal-secrets",
		},
		{
			name:      "wraps a generic API error",
			args:      []string{"--filter", "name:*"},
			searchErr: errors.New("boom"),
			wantErr:   "failed to search applications: boom",
		},
		{
			name:      "explains a search timeout",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusGatewayTimeout, nil, errors.New("timeout")),
			wantErr:   "failed to search applications: the search timed out, simplify your query and try again: 504: timeout",
		},
		{
			name:      "explains an unavailable Early Access endpoint",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusNotFound, nil, errors.New("not found")),
			wantErr:   "failed to search applications: the search endpoint is in Early Access and may not be enabled for this tenant: 404: not found",
		},
		{
			name:         "explains a search timeout in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusGatewayTimeout,
			wantErr:      "failed to search applications: the search timed out, simplify your query and try again: 504: API request failed: Gateway Timeout",
		},
		{
			name:         "explains an unavailable Early Access endpoint in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusNotFound,
			wantErr:      "failed to search applications: the search endpoint is in Early Access and may not be enabled for this tenant: 404: API request failed: Not Found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var captured *managementv3.SearchClientsRequestParameters
			searchAPI := mock.NewMockClientAPIV3(ctrl)
			if test.searchPage != nil || test.searchErr != nil {
				searchAPI.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *managementv3.SearchClientsRequestParameters, _ ...option.RequestOption) (*auth0.ClientSearchPage, error) {
						captured = req
						return test.searchPage, test.searchErr
					})
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.assertHTTP != nil {
					test.assertHTTP(t, r)
				}
				if test.serverStatus != 0 {
					w.WriteHeader(test.serverStatus)
					_, _ = w.Write([]byte(http.StatusText(test.serverStatus)))
					return
				}
				_, _ = w.Write([]byte(test.serverBody))
			}))
			defer server.Close()

			renderer := testRenderer()
			cli := &cli{
				api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
				apiv3:    &auth0.APIV3{ClientV3: searchAPI},
				renderer: renderer,
			}

			cmd := searchAppsCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			if test.assertRequest != nil {
				test.assertRequest(t, captured)
			}
			if test.assertOutput != nil {
				test.assertOutput(t, fmt.Sprint(renderer.ResultWriter), fmt.Sprint(renderer.MessageWriter))
			}
		})
	}
}

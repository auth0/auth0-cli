package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/auth0/go-auth0/management"
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

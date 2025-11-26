package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cli := &cli{}
			cli.noInput = true // non-interactive mode
			cmd := createAppCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			assert.EqualError(t, err, test.expectedError)
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

package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"github.com/auth0/go-auth0/management"
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
					[]string{"CLIENT ID", "NAME", "TYPE"},
					[][]string{
						{"some-id", "some-name", "Generic"},
					},
				)
			},
		},
		{
			name: "reveal secrets",
			args: []string{"--reveal"},
			assertOutput: func(t testing.TB, out string) {
				expectTable(t, out,
					[]string{"CLIENT ID", "NAME", "TYPE", "CLIENT SECRET"},
					[][]string{
						{"some-id", "some-name", "Generic", "secret-here"},
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

			clientAPI := auth0.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ClientList{
					Clients: []*management.Client{
						{
							Name:         auth0.String("some-name"),
							ClientID:     auth0.String("some-id"),
							Callbacks:    stringToInterfaceSlice([]string{"http://localhost"}),
							ClientSecret: auth0.String("secret-here"),
						},
					},
				}, nil)

			stdout := &bytes.Buffer{}

			// Step 2: Setup our cli context. The important bits are
			// renderer and api.
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: ioutil.Discard,
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

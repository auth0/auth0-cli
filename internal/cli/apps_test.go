package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"gopkg.in/auth0.v5/management"
)

func TestClientsListCmd(t *testing.T) {
	// Step 1: Setup our client mock for this test. We only care about
	// Clients so no need to bootstrap other bits.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientAPI := auth0.NewMockClientAPI(ctrl)
	clientAPI.EXPECT().
		List().
		Return(&management.ClientList{
			Clients: []*management.Client{
				{
					Name:      auth0.String("some-name"),
					ClientID:  auth0.String("some-id"),
					Callbacks: stringToInterfaceSlice([]string{"http://localhost"}),
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

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	expectTable(t, stdout.String(),
		[]string{"CLIENT ID", "NAME", "TYPE"},
		[][]string{
			{"some-id", "some-name", "Generic"},
		},
	)
}

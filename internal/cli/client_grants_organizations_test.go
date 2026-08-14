package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestListOrganizationsClientGrantCmd(t *testing.T) {
	terminalPage := func(organizations []*managementv3.Organization) *auth0.ClientGrantOrganizationPage {
		return &auth0.ClientGrantOrganizationPage{
			Results: organizations,
			NextPageFunc: func(context.Context) (*auth0.ClientGrantOrganizationPage, error) {
				return nil, core.ErrNoPages
			},
		}
	}

	t.Run("lists the organizations of a client grant", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantOrgAPI := mock.NewMockClientGrantOrganizationAPIV3(ctrl)
		clientGrantOrgAPI.EXPECT().
			List(gomock.Any(), "cgr_1", gomock.Any()).
			Return(terminalPage([]*managementv3.Organization{
				{
					ID:          auth0.String("org_1"),
					Name:        auth0.String("travel0"),
					DisplayName: auth0.String("Travel0"),
				},
			}), nil)

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  stdout,
			},
			apiv3: &auth0.APIV3{ClientGrantOrganization: clientGrantOrgAPI},
		}

		cmd := listOrganizationsClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1"})

		assert.NoError(t, cmd.Execute())
		expectTable(t, stdout.String(),
			[]string{"ID", "NAME", "DISPLAY NAME"},
			[][]string{
				{"org_1", "travel0", "Travel0"},
			},
		)
	})

	t.Run("renders an empty state when the grant has no organizations", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantOrgAPI := mock.NewMockClientGrantOrganizationAPIV3(ctrl)
		clientGrantOrgAPI.EXPECT().
			List(gomock.Any(), "cgr_1", gomock.Any()).
			Return(terminalPage(nil), nil)

		message := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: message,
				ResultWriter:  io.Discard,
			},
			apiv3: &auth0.APIV3{ClientGrantOrganization: clientGrantOrgAPI},
		}

		cmd := listOrganizationsClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1"})

		assert.NoError(t, cmd.Execute())
		assert.Contains(t, message.String(), "No client grant organizations available.")
	})

	t.Run("wraps an API error with the grant id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantOrgAPI := mock.NewMockClientGrantOrganizationAPIV3(ctrl)
		clientGrantOrgAPI.EXPECT().
			List(gomock.Any(), "cgr_1", gomock.Any()).
			Return(nil, errors.New("api boom"))

		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: io.Discard,
				ResultWriter:  io.Discard,
			},
			apiv3: &auth0.APIV3{ClientGrantOrganization: clientGrantOrgAPI},
		}

		cmd := listOrganizationsClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1"})

		assert.EqualError(t, cmd.Execute(), `failed to list organizations for client grant with ID "cgr_1": api boom`)
	})

	t.Run("rejects an out-of-range number", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true // Non-interactive mode.

		cmd := listOrganizationsClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1", "--number", "1001"})

		assert.EqualError(t, cmd.Execute(), "number flag invalid, please pass a number between 1 and 1000")
	})
}

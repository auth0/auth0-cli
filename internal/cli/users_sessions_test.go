package cli

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func userSessionPage(results ...*managementv3.SessionResponseContent) *auth0.UserSessionPage {
	return &auth0.UserSessionPage{
		Results: results,
		NextPageFunc: func(context.Context) (*auth0.UserSessionPage, error) {
			return nil, core.ErrNoPages
		},
	}
}

func TestListUserSessionsCmd(t *testing.T) {
	t.Run("lists the user's sessions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		userSessionAPI := mock.NewMockUserSessionAPIV3(ctrl)
		userSessionAPI.EXPECT().
			List(gomock.Any(), "user_1", gomock.Any()).
			Return(userSessionPage(
				&managementv3.SessionResponseContent{ID: auth0.String("sess_1")},
				&managementv3.SessionResponseContent{ID: auth0.String("sess_2")},
			), nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{UserSession: userSessionAPI},
			renderer: testRenderer(),
		}

		cmd := listUserSessionsCmd(cli)
		cmd.SetArgs([]string{"user_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("rejects an out-of-range number flag", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{UserSession: mock.NewMockUserSessionAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := listUserSessionsCmd(cli)
		cmd.SetArgs([]string{"user_1", "--number", "0"})

		assert.EqualError(t, cmd.Execute(), "number flag invalid, please pass a number between 1 and 1000")
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		userSessionAPI := mock.NewMockUserSessionAPIV3(ctrl)
		userSessionAPI.EXPECT().
			List(gomock.Any(), "user_1", gomock.Any()).
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{UserSession: userSessionAPI},
			renderer: testRenderer(),
		}

		cmd := listUserSessionsCmd(cli)
		cmd.SetArgs([]string{"user_1"})

		assert.EqualError(t, cmd.Execute(), `failed to list sessions for user with ID "user_1": boom`)
	})
}

func TestDeleteUserSessionsCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userSessionAPI := mock.NewMockUserSessionAPIV3(ctrl)
	userSessionAPI.EXPECT().
		Delete(gomock.Any(), "user_1").
		Return(nil)

	cli := &cli{
		apiv3:    &auth0.APIV3{UserSession: userSessionAPI},
		renderer: testRenderer(),
	}

	cmd := deleteUserSessionsCmd(cli)
	cmd.SetArgs([]string{"user_1", "--force"})

	assert.NoError(t, cmd.Execute())
}

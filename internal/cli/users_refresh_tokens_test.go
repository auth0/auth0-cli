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

func userRefreshTokenPage(results ...*managementv3.RefreshTokenResponseContent) *auth0.UserRefreshTokenPage {
	return &auth0.UserRefreshTokenPage{
		Results: results,
		NextPageFunc: func(context.Context) (*auth0.UserRefreshTokenPage, error) {
			return nil, core.ErrNoPages
		},
	}
}

func TestListUserRefreshTokensCmd(t *testing.T) {
	t.Run("lists the user's refresh tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		userRefreshTokenAPI := mock.NewMockUserRefreshTokenAPIV3(ctrl)
		userRefreshTokenAPI.EXPECT().
			List(gomock.Any(), "user_1", gomock.Any()).
			Return(userRefreshTokenPage(
				&managementv3.RefreshTokenResponseContent{ID: auth0.String("rt_1")},
				&managementv3.RefreshTokenResponseContent{ID: auth0.String("rt_2")},
			), nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{UserRefreshToken: userRefreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := listUserRefreshTokensCmd(cli)
		cmd.SetArgs([]string{"user_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("rejects an out-of-range number flag", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{UserRefreshToken: mock.NewMockUserRefreshTokenAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := listUserRefreshTokensCmd(cli)
		cmd.SetArgs([]string{"user_1", "--number", "2000"})

		assert.EqualError(t, cmd.Execute(), "number flag invalid, please pass a number between 1 and 1000")
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		userRefreshTokenAPI := mock.NewMockUserRefreshTokenAPIV3(ctrl)
		userRefreshTokenAPI.EXPECT().
			List(gomock.Any(), "user_1", gomock.Any()).
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{UserRefreshToken: userRefreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := listUserRefreshTokensCmd(cli)
		cmd.SetArgs([]string{"user_1"})

		assert.EqualError(t, cmd.Execute(), `failed to list refresh tokens for user with ID "user_1": boom`)
	})
}

func TestDeleteUserRefreshTokensCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRefreshTokenAPI := mock.NewMockUserRefreshTokenAPIV3(ctrl)
	userRefreshTokenAPI.EXPECT().
		Delete(gomock.Any(), "user_1").
		Return(nil)

	cli := &cli{
		apiv3:    &auth0.APIV3{UserRefreshToken: userRefreshTokenAPI},
		renderer: testRenderer(),
	}

	cmd := deleteUserRefreshTokensCmd(cli)
	cmd.SetArgs([]string{"user_1", "--force"})

	assert.NoError(t, cmd.Execute())
}

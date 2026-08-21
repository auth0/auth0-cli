package cli

import (
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestShowRefreshTokenCmd(t *testing.T) {
	t.Run("shows a refresh token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
		refreshTokenAPI.EXPECT().
			Get(gomock.Any(), "rt_1").
			Return(&managementv3.GetRefreshTokenResponseContent{ID: auth0.String("rt_1")}, nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := showRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"rt_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
		refreshTokenAPI.EXPECT().
			Get(gomock.Any(), "rt_1").
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := showRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"rt_1"})

		assert.EqualError(t, cmd.Execute(), `failed to read refresh token with ID "rt_1": boom`)
	})
}

func TestUpdateRefreshTokenCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var captured *managementv3.UpdateRefreshTokenRequestContent
	refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
	refreshTokenAPI.EXPECT().
		Update(gomock.Any(), "rt_1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateRefreshTokenRequestContent, _ ...option.RequestOption) (*managementv3.UpdateRefreshTokenResponseContent, error) {
			captured = req
			return &managementv3.UpdateRefreshTokenResponseContent{ID: auth0.String("rt_1")}, nil
		})

	cli := &cli{
		apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
		renderer: testRenderer(),
	}

	cmd := updateRefreshTokenCmd(cli)
	cmd.SetArgs([]string{"rt_1", "--metadata", "key=value"})

	assert.NoError(t, cmd.Execute())
	assert.NotNil(t, captured.RefreshTokenMetadata)
	assert.Equal(t, "value", (*captured.RefreshTokenMetadata)["key"])
}

func TestDeleteRefreshTokenCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
	refreshTokenAPI.EXPECT().
		Delete(gomock.Any(), "rt_1").
		Return(nil)

	cli := &cli{
		apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
		renderer: testRenderer(),
	}

	cmd := deleteRefreshTokenCmd(cli)
	cmd.SetArgs([]string{"rt_1", "--force"})

	assert.NoError(t, cmd.Execute())
}

func TestRevokeRefreshTokenCmd(t *testing.T) {
	t.Run("revokes a single token by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.RevokeRefreshTokensRequestContent
		refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
		refreshTokenAPI.EXPECT().
			Revoke(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.RevokeRefreshTokensRequestContent, _ ...option.RequestOption) error {
				captured = req
				return nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"rt_1", "--force"})

		assert.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"rt_1"}, captured.IDs)
		assert.Nil(t, captured.UserID)
	})

	t.Run("revokes all of a user's tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.RevokeRefreshTokensRequestContent
		refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
		refreshTokenAPI.EXPECT().
			Revoke(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.RevokeRefreshTokensRequestContent, _ ...option.RequestOption) error {
				captured = req
				return nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"--user-id", "user_1", "--force"})

		assert.NoError(t, cmd.Execute())
		assert.Nil(t, captured.IDs)
		assert.Equal(t, "user_1", *captured.UserID)
		assert.Nil(t, captured.ClientID)
	})

	t.Run("scopes a user revocation to client and audience", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.RevokeRefreshTokensRequestContent
		refreshTokenAPI := mock.NewMockRefreshTokenAPIV3(ctrl)
		refreshTokenAPI.EXPECT().
			Revoke(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.RevokeRefreshTokensRequestContent, _ ...option.RequestOption) error {
				captured = req
				return nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: refreshTokenAPI},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"--user-id", "user_1", "--client-id", "client_1", "--audience", "https://api", "--force"})

		assert.NoError(t, cmd.Execute())
		assert.Equal(t, "user_1", *captured.UserID)
		assert.Equal(t, "client_1", *captured.ClientID)
		assert.Equal(t, "https://api", *captured.Audience)
	})

	t.Run("rejects --client-id without --user-id", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: mock.NewMockRefreshTokenAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"--client-id", "client_1", "--force"})

		assert.EqualError(t, cmd.Execute(), "--client-id requires --user-id")
	})

	t.Run("rejects --audience without --client-id", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: mock.NewMockRefreshTokenAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"--user-id", "user_1", "--audience", "https://api", "--force"})

		assert.EqualError(t, cmd.Execute(), "--audience requires --user-id and --client-id")
	})

	t.Run("rejects both a token id and --user-id", func(t *testing.T) {
		cli := &cli{
			apiv3:    &auth0.APIV3{RefreshToken: mock.NewMockRefreshTokenAPIV3(gomock.NewController(t))},
			renderer: testRenderer(),
		}

		cmd := revokeRefreshTokenCmd(cli)
		cmd.SetArgs([]string{"rt_1", "--user-id", "user_1", "--force"})

		assert.EqualError(t, cmd.Execute(), "pass either a token id or --user-id, not both")
	})
}

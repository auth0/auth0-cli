package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func testRenderer() *display.Renderer {
	return &display.Renderer{
		MessageWriter: &bytes.Buffer{},
		ResultWriter:  &bytes.Buffer{},
	}
}

func TestShowSessionCmd(t *testing.T) {
	t.Run("shows a session", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sessionAPI := mock.NewMockSessionAPIV3(ctrl)
		sessionAPI.EXPECT().
			Get(gomock.Any(), "sess_1").
			Return(&managementv3.GetSessionResponseContent{ID: auth0.String("sess_1")}, nil)

		cli := &cli{
			apiv3:    &auth0.APIV3{Session: sessionAPI},
			renderer: testRenderer(),
		}

		cmd := showSessionCmd(cli)
		cmd.SetArgs([]string{"sess_1"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sessionAPI := mock.NewMockSessionAPIV3(ctrl)
		sessionAPI.EXPECT().
			Get(gomock.Any(), "sess_1").
			Return(nil, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{Session: sessionAPI},
			renderer: testRenderer(),
		}

		cmd := showSessionCmd(cli)
		cmd.SetArgs([]string{"sess_1"})

		assert.EqualError(t, cmd.Execute(), `failed to read session with ID "sess_1": boom`)
	})
}

func TestUpdateSessionCmd(t *testing.T) {
	t.Run("sends the metadata pairs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.UpdateSessionRequestContent
		sessionAPI := mock.NewMockSessionAPIV3(ctrl)
		sessionAPI.EXPECT().
			Update(gomock.Any(), "sess_1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateSessionRequestContent, _ ...option.RequestOption) (*managementv3.UpdateSessionResponseContent, error) {
				captured = req
				return &managementv3.UpdateSessionResponseContent{ID: auth0.String("sess_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{Session: sessionAPI},
			renderer: testRenderer(),
		}

		cmd := updateSessionCmd(cli)
		cmd.SetArgs([]string{"sess_1", "--metadata", "key=value"})

		assert.NoError(t, cmd.Execute())
		assert.NotNil(t, captured.SessionMetadata)
		assert.Equal(t, "value", (*captured.SessionMetadata)["key"])
	})

	t.Run("clears the metadata when no pairs are passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.UpdateSessionRequestContent
		sessionAPI := mock.NewMockSessionAPIV3(ctrl)
		sessionAPI.EXPECT().
			Update(gomock.Any(), "sess_1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateSessionRequestContent, _ ...option.RequestOption) (*managementv3.UpdateSessionResponseContent, error) {
				captured = req
				return &managementv3.UpdateSessionResponseContent{ID: auth0.String("sess_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{Session: sessionAPI},
			renderer: testRenderer(),
		}

		cmd := updateSessionCmd(cli)
		cmd.SetArgs([]string{"sess_1"})

		assert.NoError(t, cmd.Execute())
		assert.NotNil(t, captured.SessionMetadata)
		assert.Empty(t, *captured.SessionMetadata)
	})
}

func TestDeleteSessionCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sessionAPI := mock.NewMockSessionAPIV3(ctrl)
	sessionAPI.EXPECT().
		Delete(gomock.Any(), "sess_1").
		Return(nil)

	cli := &cli{
		apiv3:    &auth0.APIV3{Session: sessionAPI},
		renderer: testRenderer(),
	}

	cmd := deleteSessionCmd(cli)
	cmd.SetArgs([]string{"sess_1", "--force"})

	assert.NoError(t, cmd.Execute())
}

func TestRevokeSessionCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sessionAPI := mock.NewMockSessionAPIV3(ctrl)
	sessionAPI.EXPECT().
		Revoke(gomock.Any(), "sess_1").
		Return(nil)

	cli := &cli{
		apiv3:    &auth0.APIV3{Session: sessionAPI},
		renderer: testRenderer(),
	}

	cmd := revokeSessionCmd(cli)
	cmd.SetArgs([]string{"sess_1", "--force"})

	assert.NoError(t, cmd.Execute())
}

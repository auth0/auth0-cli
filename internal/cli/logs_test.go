package cli

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestTailLogsCommand(t *testing.T) {
	t.Run("it returns an error when it fails to get the logs on the first request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logsAPI := mock.NewMockLogAPI(ctrl)
		logsAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("generic error"))

		cli := &cli{
			api: &auth0.API{Log: logsAPI},
		}

		cmd := tailLogsCmd(cli)
		cmd.SetArgs([]string{"--number", "90", "--filter", "user_id:123"})
		err := cmd.Execute()

		assert.EqualError(t, err, "failed to get logs: generic error")
	})

	t.Run("it returns an error when it fails to get the logs on the 3rd request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logsAPI := mock.NewMockLogAPI(ctrl)
		logsAPI.EXPECT().
			List(gomock.Any()).
			Return(
				[]*management.Log{
					{
						LogID:       auth0.String("354234"),
						Type:        auth0.String("sapi"),
						Description: auth0.String("Update branding settings"),
					},
				},
				nil,
			)

		logsAPI.EXPECT().
			List(gomock.Any()).
			Return(
				[]*management.Log{
					{
						LogID:       auth0.String("354234"),
						Type:        auth0.String("sapi"),
						Description: auth0.String("Update branding settings"),
					},
					{
						LogID:       auth0.String("354236"),
						Type:        auth0.String("sapi"),
						Description: auth0.String("Update tenant settings"),
					},
				},
				nil,
			)

		logsAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("generic error"))

		expectedResult := `TYPE                       DESCRIPTION                                               DATE                    CONNECTION              CLIENT                  
API Operation              Update branding settings                                  Jan 01 00:00:00.000     N/A                     N/A    
`

		message := &bytes.Buffer{}
		result := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				Tenant:        "auth0-cli-tests.eu.auth0.com",
				MessageWriter: message,
				ResultWriter:  result,
			},
			api: &auth0.API{Log: logsAPI},
		}

		cmd := tailLogsCmd(cli)
		cmd.SetArgs([]string{"--number", "90", "--filter", "user_id:123"})
		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, message.String(), "auth0-cli-tests.eu.auth0.com") // Ensure we display the tenant name.
		assert.Contains(t, message.String(), "logs")                         // Ensure header is set in output.
		assert.Contains(t, message.String(), "Failed to get latest logs: generic error")
		assert.Equal(t, expectedResult, result.String())
	})
}

func TestDedupeLogs(t *testing.T) {
	t.Run("removes duplicate logs and sorts by date asc", func(t *testing.T) {
		logs := []*management.Log{
			{
				ID:   auth0.String("some-id-1"),
				Date: auth0.Time(time.Date(2023, 04, 06, 13, 00, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-2"),
				Date: auth0.Time(time.Date(2023, 04, 06, 11, 0, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-3"),
				Date: auth0.Time(time.Date(2023, 04, 06, 12, 00, 00, 0, time.UTC)),
			},
		}
		set := map[string]struct{}{"some-id-3": {}}
		result := dedupeLogs(logs, set)

		assert.Len(t, result, 2)
		assert.Equal(t, "some-id-2", result[0].GetID())
		assert.Equal(t, "some-id-1", result[1].GetID())
	})

	t.Run("does not remove any logs and sorts by date asc", func(t *testing.T) {
		logs := []*management.Log{
			{
				ID:   auth0.String("some-id-1"),
				Date: auth0.Time(time.Date(2023, 04, 06, 13, 00, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-2"),
				Date: auth0.Time(time.Date(2023, 04, 06, 11, 0, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-3"),
				Date: auth0.Time(time.Date(2023, 04, 06, 12, 00, 00, 0, time.UTC)),
			},
		}
		set := map[string]struct{}{}
		result := dedupeLogs(logs, set)

		assert.Len(t, logs, 3)
		assert.Equal(t, "some-id-2", result[0].GetID())
		assert.Equal(t, "some-id-3", result[1].GetID())
		assert.Equal(t, "some-id-1", result[2].GetID())
	})

	t.Run("removes all logs", func(t *testing.T) {
		logs := []*management.Log{
			{
				ID:   auth0.String("some-id-1"),
				Date: auth0.Time(time.Date(2023, 04, 06, 13, 00, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-2"),
				Date: auth0.Time(time.Date(2023, 04, 06, 11, 0, 00, 0, time.UTC)),
			},
			{
				ID:   auth0.String("some-id-3"),
				Date: auth0.Time(time.Date(2023, 04, 06, 12, 00, 00, 0, time.UTC)),
			},
		}
		set := map[string]struct{}{
			"some-id-1": {},
			"some-id-2": {},
			"some-id-3": {},
		}
		result := dedupeLogs(logs, set)

		assert.Len(t, result, 0)
	})
}

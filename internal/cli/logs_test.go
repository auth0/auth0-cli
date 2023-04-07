package cli

import (
	"testing"
	"time"

	"github.com/auth0/go-auth0/management"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestDedupLogs(t *testing.T) {
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
		set := map[string]struct{} { "some-id-3": {} }
		result := dedupLogs(logs, set)

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
		set := map[string]struct{} {}
		result := dedupLogs(logs, set)

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
		set := map[string]struct{} {
			"some-id-1": {},
			"some-id-2": {},
			"some-id-3": {},
		}
		result := dedupLogs(logs, set)

		assert.Len(t, result, 0)
	})
}

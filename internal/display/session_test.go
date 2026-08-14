package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDateForDisplay(t *testing.T) {
	t.Run("renders a dash for an unset date", func(t *testing.T) {
		assert.Equal(t, "-", dateForDisplay(time.Time{}))
	})

	t.Run("renders a past date as a relative time", func(t *testing.T) {
		assert.Equal(t, "an hour ago", dateForDisplay(time.Now().Add(-90*time.Minute)))
	})

	t.Run("renders a future date as an absolute date", func(t *testing.T) {
		future := time.Date(2099, time.December, 31, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "Dec 31 2099", dateForDisplay(future))
	})
}

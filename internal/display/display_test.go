package display

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func TestTimeAgo(t *testing.T) {
	t0 := time.Now()
	monthAgo := t0.Add(-(30 * 24) * time.Hour)

	tests := []struct {
		ts   time.Time
		want string
	}{
		{t0, "0 seconds ago"},
		{t0.Add(-61 * time.Second), "a minute ago"},
		{t0.Add(-2 * time.Minute), "2 minutes ago"},
		{t0.Add(-119 * time.Minute), "an hour ago"},
		{t0.Add(-3 * time.Hour), "3 hours ago"},
		{t0.Add(-23 * time.Hour), "23 hours ago"},
		{t0.Add(-24 * time.Hour), "a day ago"},
		{t0.Add(-48 * time.Hour), "2 days ago"},
		{t0.Add(-(29 * 24) * time.Hour), "29 days ago"},
		{monthAgo, monthAgo.Format("Jan 02 2006")},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			got := timeAgo(test.ts)

			if test.want != got {
				t.Fatalf("wanted %q, got %q", test.want, got)
			}
		})
	}
}

func TestStream(t *testing.T) {
	results := []View{}
	stdout := &bytes.Buffer{}
	mockRender := &Renderer{
		MessageWriter: io.Discard,
		ResultWriter:  stdout,
	}

	t.Run("Stream correctly handles nil channel", func(t *testing.T) {
		mockRender.Stream(results, nil)
		assert.Len(t, stdout.Bytes(), 0)
	})

	t.Run("Stream successfully", func(t *testing.T) {
		viewChan := make(chan View)
		go mockRender.Stream(results, viewChan)

		mockLogID := "log1"
		mockLog := management.Log{LogID: &mockLogID}
		viewChan <- &logView{Log: &mockLog}
		close(viewChan)
	})
}

func TestIndent(t *testing.T) {
	assert.Equal(t, "foo", indent("foo", ""))
	assert.Equal(t, " foo", indent("foo", " "))
	assert.Equal(t, " line1\n line2\n line3", indent("line1\nline2\nline3", " "))
}

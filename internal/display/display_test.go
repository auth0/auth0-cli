package display

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
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
	var stdout bytes.Buffer
	mockRender := &Renderer{
		MessageWriter: io.Discard,
		ResultWriter:  &stdout,
	}

	results := []View{
		&logView{
			Log: &management.Log{
				LogID:       auth0.String("354234"),
				Type:        auth0.String("sapi"),
				Description: auth0.String("Update branding settings"),
			},
		},
	}

	t.Run("Stream correctly handles nil channel", func(t *testing.T) {
		mockRender.Stream(results, nil)
		expectedResult := `TYPE                       DESCRIPTION                                               DATE                    CONNECTION              CLIENT                  
API Operation              Update branding settings                                  Jan 01 00:00:00.000     N/A                     N/A    
`
		assert.Equal(t, expectedResult, stdout.String())
		stdout.Reset()
	})

	t.Run("Stream successfully", func(t *testing.T) {
		viewChan := make(chan View)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			mockRender.Stream(results, viewChan)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			viewChan <- &logView{
				Log: &management.Log{
					LogID:       auth0.String("354236"),
					Type:        auth0.String("sapi"),
					Description: auth0.String("Update tenant settings"),
				},
			}
			close(viewChan)
		}()

		wg.Wait()

		expectedResult := `TYPE                       DESCRIPTION                                               DATE                    CONNECTION              CLIENT                  
API Operation              Update branding settings                                  Jan 01 00:00:00.000     N/A                     N/A    
API Operation              Update tenant settings                                    Jan 01 00:00:00.000     N/A                     N/A    
`
		assert.Equal(t, expectedResult, stdout.String())
		stdout.Reset()
	})
}

func TestIndent(t *testing.T) {
	assert.Equal(t, "foo", indent("foo", ""))
	assert.Equal(t, " foo", indent("foo", " "))
	assert.Equal(t, " line1\n line2\n line3", indent("line1\nline2\nline3", " "))
}

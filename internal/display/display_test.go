package display

import (
	"bytes"
	"io"
	"testing"
	"time"

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

func TestIndent(t *testing.T) {
	assert.Equal(t, "foo", indent("foo", ""))
	assert.Equal(t, " foo", indent("foo", " "))
	assert.Equal(t, " line1\n line2\n line3", indent("line1\nline2\nline3", " "))
}

func TestRenderer_Results(t *testing.T) {
	var stdout bytes.Buffer
	mockRender := &Renderer{
		MessageWriter: io.Discard,
		ResultWriter:  &stdout,
	}

	var testCases = []struct {
		name            string
		givenData       []View
		givenFormat     string
		expectedResults string
	}{
		{
			name: "it can correctly output members as a table",
			givenData: []View{
				&membersView{
					ID:    "123",
					Name:  "John",
					Email: "john@example.com",
				},
			},
			expectedResults: "  ID   NAME  EMAIL             PICTURE  \n  123  John  john@example.com           \n",
		},
		{
			name: "it can correctly output members as json",
			givenData: []View{
				&membersView{
					ID:    "123",
					Name:  "John",
					Email: "john@example.com",
					raw: struct {
						ID    string
						Name  string
						Email string
					}{
						ID:    "123",
						Name:  "John",
						Email: "john@example.com",
					},
				},
			},
			givenFormat:     string(OutputFormatJSON),
			expectedResults: "[\n    {\n        \"ID\": \"123\",\n        \"Name\": \"John\",\n        \"Email\": \"john@example.com\"\n    }\n]\n",
		},
		{
			name:            "it can correctly output an empty json array when no data",
			givenData:       []View{},
			givenFormat:     string(OutputFormatJSON),
			expectedResults: "[]\n",
		},
		{
			name: "it can correctly output members as csv",
			givenData: []View{
				&membersView{
					ID:    "123",
					Name:  "John",
					Email: "john@example.com",
				},
			},
			givenFormat:     string(OutputFormatCSV),
			expectedResults: "ID,Name,Email,Picture\n123,John,john@example.com,\n",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockRender.Format = OutputFormat(testCase.givenFormat)
			mockRender.Results(testCase.givenData)

			assert.Equal(t, testCase.expectedResults, stdout.String())
			stdout.Reset()
		})
	}
}

func TestRenderer_Stream_JSON(t *testing.T) {
	newView := func(id, name string) View {
		return &membersView{
			ID:  id,
			raw: struct{ ID, Name string }{ID: id, Name: name},
		}
	}

	t.Run("emits one compact JSON object per line (NDJSON), no header", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout, Format: OutputFormatJSON}

		r.Stream([]View{newView("1", "a"), newView("2", "b")}, nil)

		assert.Equal(t,
			"{\"ID\":\"1\",\"Name\":\"a\"}\n{\"ID\":\"2\",\"Name\":\"b\"}\n",
			stdout.String(),
		)
	})

	t.Run("streams channel records incrementally after the initial batch", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout, Format: OutputFormatJSONCompact}

		ch := make(chan View, 1)
		ch <- newView("2", "b")
		close(ch)

		r.Stream([]View{newView("1", "a")}, ch)

		assert.Equal(t,
			"{\"ID\":\"1\",\"Name\":\"a\"}\n{\"ID\":\"2\",\"Name\":\"b\"}\n",
			stdout.String(),
		)
	})

	t.Run("default (table) format is unaffected", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout}

		r.Stream([]View{newView("1", "a")}, nil)

		// Table streaming prints a header row, so the output is not JSON.
		assert.Contains(t, stdout.String(), "TYPE")
	})
}

func TestRenderer_StructuredMessages(t *testing.T) {
	t.Run("diagnostics are emitted as JSON lines on stderr", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, StructuredMessages: true}

		r.Infof("hello %s", "world")
		r.Warnf("careful")
		r.Errorf("boom")
		r.Successf("done")
		r.Detailf("more")

		assert.Equal(t,
			"{\"level\":\"info\",\"message\":\"hello world\"}\n"+
				"{\"level\":\"warning\",\"message\":\"careful\"}\n"+
				"{\"level\":\"error\",\"message\":\"boom\"}\n"+
				"{\"level\":\"success\",\"message\":\"done\"}\n"+
				"{\"level\":\"detail\",\"message\":\"more\"}\n",
			stderr.String(),
		)
	})

	t.Run("heading is suppressed in structured mode", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, StructuredMessages: true, Tenant: "example"}

		r.Heading("logs")

		assert.Empty(t, stderr.String())
	})

	t.Run("human mode is unaffected", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard}

		r.Infof("hello")

		assert.Contains(t, stderr.String(), "hello")
		assert.NotContains(t, stderr.String(), "\"level\"")
	})
}

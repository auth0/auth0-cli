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

	t.Run("json-compact emits one compact JSON object per line (NDJSON), no header", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout, Format: OutputFormatJSONCompact}

		r.Stream([]View{newView("1", "a"), newView("2", "b")}, nil)

		assert.Equal(t,
			"{\"ID\":\"1\",\"Name\":\"a\"}\n{\"ID\":\"2\",\"Name\":\"b\"}\n",
			stdout.String(),
		)
	})

	t.Run("json emits each record as an indented object", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout, Format: OutputFormatJSON}

		r.Stream([]View{newView("1", "a"), newView("2", "b")}, nil)

		assert.Equal(t,
			"{\n  \"ID\": \"1\",\n  \"Name\": \"a\"\n}\n{\n  \"ID\": \"2\",\n  \"Name\": \"b\"\n}\n",
			stdout.String(),
		)
	})

	t.Run("agent mode keeps the compact NDJSON contract even with json", func(t *testing.T) {
		var stdout bytes.Buffer
		r := &Renderer{MessageWriter: io.Discard, ResultWriter: &stdout, Format: OutputFormatJSON, AgentMode: true}

		r.Stream([]View{newView("1", "a")}, nil)

		assert.Equal(t, "{\"ID\":\"1\",\"Name\":\"a\"}\n", stdout.String())
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

func TestRenderer_OutputPreformattedJSON(t *testing.T) {
	// `auth0 api` output is JSON in every format, so it must always end with a
	// trailing newline on stdout so a piped reader never drops the final line.
	for _, format := range []OutputFormat{"", OutputFormatJSON, OutputFormatJSONCompact} {
		t.Run("terminates with a newline in format "+string(format), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			r := &Renderer{MessageWriter: &stderr, ResultWriter: &stdout, Format: format}

			r.OutputPreformattedJSON("[]")

			assert.Equal(t, "[]\n", stdout.String())
			assert.Empty(t, stderr.String(), "the result must not leak onto stderr")
		})
	}
}

func TestRenderer_AgentMode(t *testing.T) {
	t.Run("human diagnostics are suppressed on stderr", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, AgentMode: true}

		r.Infof("hello %s", "world")
		r.Warnf("careful")
		r.Errorf("boom")
		r.Successf("done")
		r.Detailf("more")

		// Agent mode keeps stderr clean: the only thing that ever lands there is
		// the JSON error envelope on failure, so a merged stream is parseable.
		assert.Empty(t, stderr.String())
	})

	t.Run("heading is suppressed in agent mode", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, AgentMode: true, Tenant: "example"}

		r.Heading("logs")

		assert.Empty(t, stderr.String())
	})

	t.Run("blank line is suppressed in agent mode and JSON modes", func(t *testing.T) {
		for _, r := range []*Renderer{
			{AgentMode: true},
			{Format: OutputFormatJSON},
			{Format: OutputFormatJSONCompact},
		} {
			var stderr bytes.Buffer
			r.MessageWriter = &stderr
			r.ResultWriter = io.Discard

			r.Newline()

			assert.Empty(t, stderr.String())
		}
	})

	t.Run("blank line is written in the default text mode", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard}

		r.Newline()

		assert.Equal(t, "\n", stderr.String())
	})

	t.Run("heading is suppressed in JSON output modes", func(t *testing.T) {
		for _, format := range []OutputFormat{OutputFormatJSON, OutputFormatJSONCompact} {
			var stderr bytes.Buffer
			r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, Format: format, Tenant: "example"}

			r.Heading("logs")

			assert.Emptyf(t, stderr.String(), "expected no heading for format %q", format)
		}
	})

	t.Run("heading is written in the default text mode", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard, Tenant: "example"}

		r.Heading("logs")

		assert.Contains(t, stderr.String(), "example")
		assert.Contains(t, stderr.String(), "logs")
	})

	t.Run("human mode is unaffected", func(t *testing.T) {
		var stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: io.Discard}

		r.Infof("hello")

		assert.Contains(t, stderr.String(), "hello")
		assert.NotContains(t, stderr.String(), "\"level\"")
	})
}

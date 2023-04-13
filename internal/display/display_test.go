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
			expectedResults: "[\n    {\n        \"ID\": \"123\",\n        \"Name\": \"John\",\n        \"Email\": \"john@example.com\"\n    }\n]",
		},
		{
			name:            "it can correctly output an empty json array when no data",
			givenData:       []View{},
			givenFormat:     string(OutputFormatJSON),
			expectedResults: "[]",
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

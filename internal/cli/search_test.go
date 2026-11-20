package cli

import (
	"bytes"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestSearchFilterFlag(t *testing.T) {
	flag := searchFilterFlag([]string{"id", "name", "metadata.{key}"}, `name:"My App"`, `name co "billing"`)

	assert.Equal(t, "filter", flag.LongForm)
	assert.Equal(t,
		"Filter expression, sent as the 'q' query parameter. Uses Lucene syntax, or SCIM syntax with --parser scim. "+
			"Supported fields: id,name,metadata.{key}. Maximum 5 filter operations. "+
			"Equivalent to --query '{\"q\":\"<expression>\"}'.\n\n"+
			`For example: 'name:"My App"' or, with --parser scim, 'name co "billing"'.`,
		flag.Help,
	)
}

func TestSearchFieldsFlag(t *testing.T) {
	flag := searchFieldsFlag("id", "name", "identifier")

	assert.Equal(t, "fields", flag.LongForm)
	assert.Equal(t, "Comma-separated list of fields to include in the response, e.g. 'id,name,identifier'.", flag.Help)
}

func TestSearchSortFlag(t *testing.T) {
	flag := searchSortFlag("name", "updated_at")

	assert.Equal(t, "sort", flag.LongForm)
	assert.Equal(t, "s", flag.ShortForm)
	assert.Equal(t,
		"Field to sort by, in ascending order only. One of: name,updated_at. "+
			"The 'field:1' form is also accepted. Defaults to insertion order.",
		flag.Help,
	)
}

func TestParseSearchInputs(t *testing.T) {
	lucene := managementv3.SearchParserEnumLucene.Ptr()

	tests := []struct {
		name    string
		inputs  searchInputs
		want    searchParams
		wantErr string
	}{
		{
			name:   "defaults to lucene and omits unset params",
			inputs: searchInputs{Number: 50},
			want:   searchParams{Parser: lucene, Take: auth0.Int(50)},
		},
		{
			name:   "maps every shared flag",
			inputs: searchInputs{Filter: `name co "billing"`, Parser: "scim", Fields: "id,name", ExcludeFields: true, Number: 20},
			want: searchParams{
				Q:             auth0.String(`name co "billing"`),
				Parser:        managementv3.SearchParserEnumSCIM.Ptr(),
				Fields:        auth0.String("id,name"),
				IncludeFields: auth0.Bool(false),
				Take:          auth0.Int(20),
			},
		},
		{
			name:   "caps the page size at 100",
			inputs: searchInputs{Number: 1000},
			want:   searchParams{Parser: lucene, Take: auth0.Int(100)},
		},
		{
			name:   "trims and normalizes the fields list",
			inputs: searchInputs{Fields: " id, name ,,identifier ", Number: 50},
			want:   searchParams{Fields: auth0.String("id,name,identifier"), Parser: lucene, Take: auth0.Int(50)},
		},
		{
			name:    "rejects an unknown parser",
			inputs:  searchInputs{Parser: "sql", Number: 50},
			wantErr: `invalid --parser "sql": must be 'lucene' or 'scim'`,
		},
		{
			name:    "rejects --exclude-fields without --fields",
			inputs:  searchInputs{ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
		{
			name:    "rejects a --filter over 1000 characters",
			inputs:  searchInputs{Filter: strings.Repeat("a", 1001), Number: 50},
			wantErr: "--filter must be at most 1000 characters",
		},
		{
			name:   "counts --filter length in characters, not bytes",
			inputs: searchInputs{Filter: strings.Repeat("界", 400), Number: 50},
			want:   searchParams{Q: auth0.String(strings.Repeat("界", 400)), Parser: lucene, Take: auth0.Int(50)},
		},
		{
			name:   "measures --fields after trimming",
			inputs: searchInputs{Fields: strings.Repeat(" ,", 600) + "id", Number: 50},
			want:   searchParams{Fields: auth0.String("id"), Parser: lucene, Take: auth0.Int(50)},
		},
		{
			name:   "treats a blank --fields as unset",
			inputs: searchInputs{Fields: " , ,", Number: 50},
			want:   searchParams{Parser: lucene, Take: auth0.Int(50)},
		},
		{
			name:    "rejects --exclude-fields with a blank --fields",
			inputs:  searchInputs{Fields: " , ", ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
		{
			name:    "rejects --fields over 1000 characters",
			inputs:  searchInputs{Fields: strings.Repeat("a", 1001), Number: 50},
			wantErr: "--fields must be at most 1000 characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSearchInputs(test.inputs)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestParseSearchSort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sortable []string
		want     string
		wantErr  string
	}{
		{name: "accepts a sortable field", input: "name", sortable: []string{"name", "updated_at"}, want: "name"},
		{name: "accepts the ascending form", input: "updated_at:1", sortable: []string{"name", "updated_at"}, want: "updated_at"},
		{
			name:     "rejects descending order",
			input:    "name:-1",
			sortable: []string{"name", "updated_at"},
			wantErr:  `invalid --sort "name:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`,
		},
		{
			name:     "lists two choices",
			input:    "created_at",
			sortable: []string{"name", "updated_at"},
			wantErr:  `invalid --sort "created_at": must be one of name,updated_at`,
		},
		{
			name:     "lists three choices",
			input:    "created_at",
			sortable: []string{"name", "identifier", "updated_at"},
			wantErr:  `invalid --sort "created_at": must be one of name,identifier,updated_at`,
		},
		{
			name:     "lists a single choice",
			input:    "name",
			sortable: []string{"domain"},
			wantErr:  `invalid --sort "name": must be one of domain`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSearchSort(test.input, test.sortable...)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestValidateSearchNumber(t *testing.T) {
	for _, number := range []int{1, 50, 1000} {
		assert.NoError(t, validateSearchNumber(number))
	}

	for _, number := range []int{0, -1, 1001} {
		assert.EqualError(t, validateSearchNumber(number), "number flag invalid, please pass a number between 1 and 1000")
	}
}

func TestValidateSearchCSV(t *testing.T) {
	columns := display.TableColumns{{Field: "id", Header: "ID"}, {Field: "name", Header: "Name"}}

	assert.NoError(t, validateSearchCSV(&cli{csv: false}, columns, []string{"other"}, false), "only --csv is checked")
	assert.NoError(t, validateSearchCSV(&cli{csv: true}, columns, []string{"name"}, false))
	assert.NoError(t, validateSearchCSV(&cli{csv: true}, columns, nil, false))
	assert.EqualError(t,
		validateSearchCSV(&cli{csv: true}, columns, []string{"other"}, false),
		"--csv needs at least one table column (id, name) left by --fields",
	)
	assert.EqualError(t,
		validateSearchCSV(&cli{csv: true}, columns, []string{"id", "name"}, true),
		"--csv needs at least one table column (id, name) left by --fields",
	)
}

func TestWarnSearchLimit(t *testing.T) {
	tests := []struct {
		name    string
		results int
		number  int
		want    string
	}{
		{name: "stays silent below --number", results: 2, number: 3},
		{name: "suggests raising --number", results: 3, number: 3, want: "Refine --filter or raise --number (max 1000)."},
		{name: "only suggests refining at the maximum", results: 1000, number: 1000, want: "more may match. Refine --filter."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var messages bytes.Buffer
			cli := &cli{renderer: &display.Renderer{MessageWriter: &messages, ResultWriter: &bytes.Buffer{}}}

			warnSearchLimit(cli, test.results, test.number)

			if test.want == "" {
				assert.Empty(t, messages.String())
				return
			}
			assert.Contains(t, messages.String(), test.want)
		})
	}
}

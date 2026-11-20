package display

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testTableColumns = TableColumns{
	{Field: "id", Header: "ID"},
	{Field: "name", Header: "Name"},
	{Field: "type", Header: "Type"},
}

func TestTableColumns_Fields(t *testing.T) {
	assert.Equal(t, []string{"id", "name", "type"}, testTableColumns.Fields())
}

func TestTableColumns_Left(t *testing.T) {
	tests := []struct {
		name    string
		fields  []string
		exclude bool
		want    []string
	}{
		{name: "keeps every column without fields", want: []string{"id", "name", "type"}},
		{name: "keeps the included columns in table order", fields: []string{"type", "id", "other"}, want: []string{"id", "type"}},
		{name: "drops the excluded columns", fields: []string{"name"}, exclude: true, want: []string{"id", "type"}},
		{name: "leaves no column when none is included", fields: []string{"other"}},
		{name: "leaves no column when all are excluded", fields: []string{"id", "name", "type"}, exclude: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, testTableColumns.Left(test.fields, test.exclude))
		})
	}
}

func TestTableColumns_LeftWithExtraFields(t *testing.T) {
	columns := TableColumns{
		{Field: "id", Header: "ID"},
		{Field: "scope", Header: "Scopes", Also: []string{"allow_all_scopes"}},
	}

	tests := []struct {
		name    string
		fields  []string
		exclude bool
		want    []string
	}{
		{name: "keeps the column for its main field", fields: []string{"scope"}, want: []string{"scope"}},
		{name: "keeps the column for an extra field alone", fields: []string{"allow_all_scopes"}, want: []string{"scope"}},
		{name: "keeps the column when only its main field is excluded", fields: []string{"scope"}, exclude: true, want: []string{"id", "scope"}},
		{name: "drops the column when all its fields are excluded", fields: []string{"scope", "allow_all_scopes"}, exclude: true, want: []string{"id"}},
		{name: "lists the column once with both fields", fields: []string{"scope", "allow_all_scopes"}, want: []string{"scope"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, columns.Left(test.fields, test.exclude))
		})
	}

	assert.Equal(t, []string{"id", "scope"}, columns.Fields(), "Fields lists only the main fields")
}

func TestTableColumns_HeadersAndRow(t *testing.T) {
	values := map[string]string{"id": "1", "name": "a", "type": "spa"}

	assert.Equal(t, []string{"ID", "Name", "Type"}, testTableColumns.headers(nil))
	assert.Equal(t, []string{"1", "a", "spa"}, testTableColumns.row(nil, values))

	assert.Equal(t, []string{"ID", "Type"}, testTableColumns.headers([]string{"type", "id"}))
	assert.Equal(t, []string{"1", "spa"}, testTableColumns.row([]string{"type", "id"}, values))
}

func TestRenderer_SearchJSONFallback(t *testing.T) {
	results := []map[string]string{{"description": "x"}}

	tests := []struct {
		name   string
		left   []string
		count  int
		format OutputFormat
		want   bool
	}{
		{name: "renders JSON when no column is left", count: 1, want: true},
		{name: "keeps the table when a column is left", left: []string{"id"}, count: 1},
		{name: "keeps the empty state when there are no results"},
		{name: "keeps an explicit output format", count: 1, format: OutputFormatJSON},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			r := &Renderer{MessageWriter: &stderr, ResultWriter: &stdout, Format: test.format}

			assert.Equal(t, test.want, r.searchJSONFallback(testTableColumns, test.left, test.count, results))

			if !test.want {
				assert.Empty(t, stdout.String())
				return
			}
			assert.Contains(t, stderr.String(), "No table columns (id, name, type) are left in the response; showing JSON instead.")
			assert.Contains(t, stdout.String(), `"description": "x"`)
		})
	}
}

func TestRenderer_TableResults(t *testing.T) {
	t.Run("shows the empty state with its hint", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: &stdout}

		r.tableResults("applications", searchEmptyHint, nil)

		assert.Contains(t, stderr.String(), "applications (0)")
		assert.Contains(t, stderr.String(), searchEmptyHint)
		assert.Empty(t, stdout.String())
	})

	t.Run("shows the count and the rows", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		r := &Renderer{MessageWriter: &stderr, ResultWriter: &stdout}

		r.tableResults("apis", searchEmptyHint, []View{
			&apiTableView{ID: "rs_1", Name: "Orders API", Identifier: "https://orders"},
		})

		assert.Contains(t, stderr.String(), "apis (1)")
		assert.Contains(t, stdout.String(), "rs_1")
		assert.Contains(t, stdout.String(), "Orders API")
	})
}

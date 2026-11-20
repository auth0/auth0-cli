package display

import (
	"fmt"
	"slices"
	"strings"
)

// searchEmptyHint is the empty-state hint of the search commands.
const searchEmptyHint = "Try a broader --filter; results may lag very recent writes"

// TableColumn is a table column and the API field it shows.
type TableColumn struct {
	Field  string
	Header string
}

// TableColumns lists the columns of a table in display order.
type TableColumns []TableColumn

// Fields returns the API field of every column.
func (c TableColumns) Fields() []string {
	fields := make([]string, 0, len(c))
	for _, column := range c {
		fields = append(fields, column.Field)
	}

	return fields
}

// Left returns the fields of the columns left in the response by a --fields list
// (or, with exclude, by --exclude-fields). Empty fields keeps every column.
func (c TableColumns) Left(fields []string, exclude bool) []string {
	var left []string
	for _, column := range c {
		if len(fields) == 0 || slices.Contains(fields, column.Field) != exclude {
			left = append(left, column.Field)
		}
	}

	return left
}

// headers returns the headers of the columns in shown. Empty shown keeps every column.
func (c TableColumns) headers(shown []string) []string {
	var headers []string
	for _, column := range c {
		if shows(shown, column.Field) {
			headers = append(headers, column.Header)
		}
	}

	return headers
}

// row returns the values, keyed by API field, of the columns in shown.
// Empty shown keeps every column.
func (c TableColumns) row(shown []string, values map[string]string) []string {
	var row []string
	for _, column := range c {
		if shows(shown, column.Field) {
			row = append(row, values[column.Field])
		}
	}

	return row
}

func shows(shown []string, field string) bool {
	return len(shown) == 0 || slices.Contains(shown, field)
}

// searchJSONFallback renders results as JSON when --fields leaves none of the table's
// columns and the output is a table, as the table would be empty. It reports whether
// it rendered the results.
func (r *Renderer) searchJSONFallback(columns TableColumns, left []string, count int, results interface{}) bool {
	if len(left) > 0 || count == 0 || r.Format != "" {
		return false
	}

	r.Infof("No table columns (%s) are left in the response; showing JSON instead.", strings.Join(columns.Fields(), ", "))
	r.JSONResult(results)

	return true
}

// tableResults renders views as a table under a "<resource> (<count>)" heading, or
// an empty state with emptyHint when there are none.
func (r *Renderer) tableResults(resource, emptyHint string, views []View) {
	r.Heading(fmt.Sprintf("%s (%d)", resource, len(views)))

	if len(views) == 0 {
		r.EmptyState(resource, emptyHint)
		return
	}

	r.Results(views)
}

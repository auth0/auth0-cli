package cli

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/auth0/go-auth0"
	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/display"
)

const (
	searchPageSize  = 100  // Max `take` per request.
	searchMaxLength = 1000 // Max length of the `q` and `fields` params.
	searchMaxNumber = 1000 // Max value of --number.
)

var (
	searchParser = Flag{
		Name:     "Parser",
		LongForm: "parser",
		Help:     "Syntax of --filter: 'lucene' or 'scim'.",
	}
	searchExcludeFields = Flag{
		Name:     "Exclude Fields",
		LongForm: "exclude-fields",
		Help:     "Exclude the --fields list from the response instead of returning only those fields.",
	}
)

// searchFilterFlag returns the --filter flag of a search command for its filterable
// fields, with an example filter in each syntax.
func searchFilterFlag(fields []string, luceneExample, scimExample string) Flag {
	return Flag{
		Name:     "Filter",
		LongForm: "filter",
		Help: "Filter expression, sent as the 'q' query parameter. Uses Lucene syntax, or SCIM syntax with --parser scim. " +
			"Supported fields: " + strings.Join(fields, ",") + ". Maximum 5 filter operations. Equivalent to --query '{\"q\":\"<expression>\"}'.\n\n" +
			"For example: '" + luceneExample + "' or, with --parser scim, '" + scimExample + "'.",
	}
}

// searchFieldsFlag returns the --fields flag of a search command, with a few of the
// fields its endpoint returns as an example.
func searchFieldsFlag(examples ...string) Flag {
	return Flag{
		Name:     "Fields",
		LongForm: "fields",
		Help:     "Comma-separated list of fields to include in the response, e.g. '" + strings.Join(examples, ",") + "'.",
	}
}

// searchSortFlag returns the --sort flag of a search command for its sortable fields.
func searchSortFlag(sortable ...string) Flag {
	return Flag{
		Name:      "Sort",
		LongForm:  "sort",
		ShortForm: "s",
		Help: "Field to sort by, in ascending order only. One of: " + strings.Join(sortable, ",") + ". " +
			"The 'field:1' form is also accepted. Defaults to insertion order.",
	}
}

// searchInputs holds the typed-mode flags of a search command.
type searchInputs struct {
	Filter        string
	Parser        string
	Sort          string
	Fields        string
	ExcludeFields bool
	Number        int
}

// searchParams holds the validated values that every search request shares. Each
// command copies them onto its own SDK request type, and maps --sort onto its enum.
type searchParams struct {
	Q             *string
	Parser        *managementv3.SearchParserEnum
	Fields        *string
	IncludeFields *bool
	Take          *int
}

// parseSearchInputs validates the typed-mode flags shared by the search commands.
func parseSearchInputs(inputs searchInputs) (searchParams, error) {
	fields := strings.Join(parseSearchFields(inputs.Fields), ",")

	// The API caps both at 1000 characters; reject early with a clearer error than its 400.
	if utf8.RuneCountInString(inputs.Filter) > searchMaxLength {
		return searchParams{}, validationError{err: fmt.Errorf("--filter must be at most %d characters", searchMaxLength)}
	}
	if utf8.RuneCountInString(fields) > searchMaxLength {
		return searchParams{}, validationError{err: fmt.Errorf("--fields must be at most %d characters", searchMaxLength)}
	}
	if inputs.ExcludeFields && fields == "" {
		return searchParams{}, validationError{err: fmt.Errorf("--exclude-fields requires --fields")}
	}

	params := searchParams{
		Take: auth0.Int(min(inputs.Number, searchPageSize)),
	}

	switch inputs.Parser {
	case "", "lucene":
		params.Parser = managementv3.SearchParserEnumLucene.Ptr()
	case "scim":
		params.Parser = managementv3.SearchParserEnumSCIM.Ptr()
	default:
		return searchParams{}, validationError{err: fmt.Errorf("invalid --parser %q: must be 'lucene' or 'scim'", inputs.Parser)}
	}

	if inputs.Filter != "" {
		params.Q = auth0.String(inputs.Filter)
	}

	if fields != "" {
		params.Fields = auth0.String(fields)
	}

	if inputs.ExcludeFields {
		params.IncludeFields = auth0.Bool(false)
	}

	return params, nil
}

// parseSearchFields splits a --fields value into trimmed, non-empty field names.
func parseSearchFields(fields string) []string {
	var names []string
	for name := range strings.SplitSeq(fields, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}

	return names
}

// parseSearchSort validates a --sort value of 'field' or 'field:1' against the sortable
// fields and returns the field. The search endpoints only sort ascending, so 'field:-1'
// is rejected.
func parseSearchSort(s string, sortable ...string) (string, error) {
	field, order, hasOrder := strings.Cut(s, ":")
	if hasOrder && order != "1" {
		return "", validationError{err: fmt.Errorf("invalid --sort %q: the search endpoint only sorts ascending, use 'field' or 'field:1'", s)}
	}

	if !slices.Contains(sortable, field) {
		return "", validationError{err: fmt.Errorf("invalid --sort %q: must be one of %s", s, strings.Join(sortable, ","))}
	}

	return field, nil
}

// validateSearchNumber rejects a --number outside 1 to 1000.
func validateSearchNumber(number int) error {
	if number < 1 || number > searchMaxNumber {
		return validationError{err: fmt.Errorf("number flag invalid, please pass a number between 1 and %d", searchMaxNumber)}
	}

	return nil
}

// validateSearchCSV rejects --csv when --fields leaves none of the table's columns in
// the response, as the CSV output would have no columns.
func validateSearchCSV(cli *cli, columns display.TableColumns, fields []string, exclude bool) error {
	if cli.csv && len(columns.Left(fields, exclude)) == 0 {
		return validationError{err: fmt.Errorf("--csv needs at least one table column (%s) left by --fields", strings.Join(columns.Fields(), ", "))}
	}

	return nil
}

// warnSearchLimit warns when the results filled --number, as more results may match.
func warnSearchLimit(cli *cli, results, number int) {
	if results != number {
		return
	}

	hint := fmt.Sprintf("Refine --filter or raise --number (max %d).", searchMaxNumber)
	if number == searchMaxNumber {
		hint = "Refine --filter."
	}

	cli.renderer.Warnf("Results may be limited by --number; more may match. %s", hint)
}

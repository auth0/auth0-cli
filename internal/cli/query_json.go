package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var listQueryFlag = Flag{
	Name:      "Query",
	LongForm:  "query",
	ShortForm: "q",
	Help:      "Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters.",
}

// jsonQuerySpec describes a list operation driven by a --query JSON payload.
type jsonQuerySpec struct {
	Path      string // API path segments (e.g. "actions/actions").
	SchemaCmd string
}

// encodeQueryParams turns a decoded JSON object of query parameters into URL
// query values with real JSON-to-query semantics, so structured filters build the
// request an agent actually intends:
//   - a scalar (string, number, bool) becomes a single value;
//   - an array of scalars becomes repeated params (?k=a&k=b), which is how the
//     Management API expects multi-valued filters, instead of the old
//     fmt.Sprintf("%v", …) that rendered an array as the literal "[a b]";
//   - a nested object or an array containing objects/arrays is rejected, since
//     there is no unambiguous query encoding for it (better a clear error than a
//     silently wrong request).
//
// Numbers must arrive as json.Number (decode with UseNumber) so their original
// literal is preserved.
func encodeQueryParams(query url.Values, params map[string]interface{}) (url.Values, error) {
	for key, val := range params {
		switch v := val.(type) {
		case []interface{}:
			for _, item := range v {
				s, err := queryScalarString(key, item)
				if err != nil {
					return nil, err
				}
				query.Add(key, s)
			}
		default:
			s, err := queryScalarString(key, v)
			if err != nil {
				return nil, err
			}
			query.Add(key, s)
		}
	}

	return query, nil
}

// queryScalarString renders a single JSON scalar as a query-parameter string. A
// nested object or array is not a scalar and is rejected, naming the offending key.
func queryScalarString(key string, val interface{}) (string, error) {
	switch v := val.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case json.Number:
		return v.String(), nil
	default:
		return "", fmt.Errorf(
			"query parameter %q must be a scalar or an array of scalars; "+
				"nested objects and arrays are not supported", key,
		)
	}
}

// runJSONQuery executes a GET request against the Management API with query parameters parsed from queryJSON.
func runJSONQuery(cli *cli, cmd *cobra.Command, spec jsonQuerySpec, queryJSON string) error {
	// UseNumber keeps numeric filters as their original literal (e.g. "5", not
	// "5" reformatted through float64, which would turn 1000000 into "1e+06").
	decoder := json.NewDecoder(strings.NewReader(queryJSON))
	decoder.UseNumber()

	var queryParams map[string]interface{}
	if err := decoder.Decode(&queryParams); err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected query parameters.", spec.SchemaCmd)
		return fmt.Errorf("invalid --query value: must be a JSON object: %w", err)
	}

	u, err := url.Parse(cli.api.HTTPClient.URI(strings.Split(spec.Path, "/")...))
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	query, err := encodeQueryParams(u.Query(), queryParams)
	if err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected query parameters.", spec.SchemaCmd)
		return fmt.Errorf("invalid --query value: %w", err)
	}
	u.RawQuery = query.Encode()

	var response *http.Response
	if err := ansi.Waiting(func() error {
		request, err := cli.api.HTTPClient.NewRequest(cmd.Context(), http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		response, err = cli.api.HTTPClient.Do(request)
		return err
	}); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	rawJSON, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return newAPIResponseError(response.StatusCode, response.Header, rawJSON)
	}

	// A single --query call fetches one page. If the envelope reports more
	// records than were returned, tell the caller so they don't mistake a page
	// for the whole result set; the output itself is left untouched.
	if hint := paginationHint(rawJSON); hint != "" {
		cli.renderer.Warnf("%s", hint)
	}

	// Honor --json-compact by emitting a single dense line; otherwise pretty-print.
	// --csv is intentionally not supported here: the response is raw API JSON with
	// no fixed column shape to flatten.
	if cli.jsonCompact {
		var compactJSON bytes.Buffer
		if err := json.Compact(&compactJSON, rawJSON); err != nil {
			return fmt.Errorf("failed to format response: %w", err)
		}
		cli.renderer.Output(compactJSON.String())
		return nil
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, rawJSON, "", "  "); err != nil {
		return fmt.Errorf("failed to format response: %w", err)
	}
	cli.renderer.Output(ansi.ColorizeJSON(prettyJSON.String()))
	return nil
}

// paginationHint returns a diagnostic when the response shows that more records
// exist than were returned on this page, or "" when the response is complete (or
// carries no pagination metadata to reason about). It covers all of the
// Management API's pagination models rather than a single envelope shape:
//
//   - Checkpoint pagination returns a "next" token (and no "total"); a non-empty
//     token means there are further pages, fetched by passing it as "from".
//   - Offset pagination returns a numeric "total"; when the number of records
//     actually returned (plus the page's "start" offset, if present) is short of
//     "total", further pages exist. This handles both the "include_totals"
//     envelope (start/limit/total) and endpoints that report only "total".
//
// A bare array or any body without "next"/"total" yields no hint, because there
// is then no reliable signal that the result set was truncated.
func paginationHint(rawJSON []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(rawJSON))
	decoder.UseNumber()

	var envelope map[string]interface{}
	if err := decoder.Decode(&envelope); err != nil {
		// A bare array or any non-object body carries no pagination metadata.
		return ""
	}

	// Checkpoint pagination: a non-empty "next" token means more pages exist.
	if next, ok := envelope["next"].(string); ok && next != "" {
		return "This is one page of a larger result set (checkpoint pagination). " +
			"More results exist; pass \"from\" set to the response's \"next\" token " +
			"(and optionally \"take\") in --query to fetch the next page."
	}

	// Offset pagination: compare records returned against the reported total.
	total, hasTotal := jsonNumberInt(envelope["total"])
	if !hasTotal {
		return ""
	}
	start, _ := jsonNumberInt(envelope["start"]) // Absent "start" means offset 0.
	returned := longestArrayLen(envelope)
	if start+returned >= total {
		return ""
	}

	return fmt.Sprintf(
		"Showing %d of %d results (page starts at %d). More results exist; "+
			"pass a higher \"page\" or \"per_page\" in --query to fetch the rest.",
		returned, total, start,
	)
}

// longestArrayLen returns the length of the longest array-valued field in the
// envelope. That field is the resource collection, since pagination metadata
// ("total", "start", "limit", "next", …) is always scalar.
func longestArrayLen(envelope map[string]interface{}) int {
	longest := 0
	for _, v := range envelope {
		if arr, ok := v.([]interface{}); ok && len(arr) > longest {
			longest = len(arr)
		}
	}
	return longest
}

// jsonNumberInt reports the integer value of a decoded JSON field when it is a
// json.Number holding an integer, and false when the field is absent or not an
// integer number.
func jsonNumberInt(val interface{}) (int, bool) {
	n, ok := val.(json.Number)
	if !ok {
		return 0, false
	}
	v, err := n.Int64()
	if err != nil {
		return 0, false
	}
	return int(v), true
}

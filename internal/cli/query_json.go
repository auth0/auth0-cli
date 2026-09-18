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
	Help:      "Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add \"include_totals\":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).",
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
//     silently wrong request);
//   - a JSON null omits the parameter rather than sending an empty value, so
//     {"page":null} drops the key instead of building "?page=".
//
// Numbers must arrive as json.Number (decode with UseNumber) so their original
// literal is preserved.
func encodeQueryParams(query url.Values, params map[string]interface{}) (url.Values, error) {
	for key, val := range params {
		switch v := val.(type) {
		case nil:
			continue
		case []interface{}:
			for _, item := range v {
				if item == nil {
					continue
				}
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
// Callers skip JSON null before reaching here (see encodeQueryParams).
func queryScalarString(key string, val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case json.Number:
		return v.String(), nil
	case float64:
		// The sole caller decodes with UseNumber, so numbers arrive as
		// json.Number. This keeps encodeQueryParams correct for a future caller
		// that decodes without UseNumber: 'f' formatting avoids scientific
		// notation, so 1000000 stays "1000000".
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf(
			"query parameter %q must be a scalar or an array of scalars; "+
				"nested objects and arrays are not supported", key,
		)
	}
}

// runJSONQuery executes a GET request against the Management API with query parameters parsed from queryJSON.
func runJSONQuery(cli *cli, cmd *cobra.Command, spec jsonQuerySpec, queryJSON string) error {
	// --csv has no meaning here: the response is raw API JSON with no fixed column
	// shape to flatten. Fail loudly instead of silently ignoring the flag.
	if cli.csv {
		return fmt.Errorf("--csv is not supported with --query: the response is raw API JSON with no fixed columns to flatten; use --json or --json-compact instead")
	}

	// UseNumber keeps numeric filters as their original literal (e.g. "5", not
	// "5" reformatted through float64, which would turn 1000000 into "1e+06").
	decoder := json.NewDecoder(strings.NewReader(queryJSON))
	decoder.UseNumber()

	var queryParams map[string]interface{}
	if err := decoder.Decode(&queryParams); err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected query parameters.", spec.SchemaCmd)
		return validationError{fmt.Errorf("invalid --query value: must be a JSON object: %w", err)}
	}

	u, err := url.Parse(cli.api.HTTPClient.URI(strings.Split(spec.Path, "/")...))
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	query, err := encodeQueryParams(u.Query(), queryParams)
	if err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected query parameters.", spec.SchemaCmd)
		return validationError{fmt.Errorf("invalid --query value: %w", err)}
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
	// (--csv is rejected up front.)
	if cli.jsonCompact {
		var compactJSON bytes.Buffer
		if err := json.Compact(&compactJSON, rawJSON); err != nil {
			return fmt.Errorf("failed to format response: %w", err)
		}
		cli.renderer.Output(ansi.ColorizeJSON(compactJSON.String()))
		return nil
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, rawJSON, "", "  "); err != nil {
		return fmt.Errorf("failed to format response: %w", err)
	}
	cli.renderer.OutputPreformattedJSON(ansi.ColorizeJSON(prettyJSON.String()))
	return nil
}

// paginationHint returns a diagnostic when the response shows that more records
// exist than were returned on this page, or "" when the response is complete (or
// carries no pagination metadata to reason about).
//
// It reads the Management API's standard list envelope, whose fields go-auth0
// models as management.List (start/limit/length/total/next), and mirrors that
// type's own HasNext() contract so the hint agrees with how the SDK defines
// "more pages":
//
//   - Checkpoint pagination: a non-empty "next" token means the SDK would attempt
//     another fetch, so more results may exist. The hint is suppressed on an empty
//     page ("length" == 0) so it never points onward from a page that returned
//     nothing. The wording stays tentative ("may be more") because a final page can
//     still carry a "next" token that yields an empty page when followed.
//   - Offset pagination: more pages exist when "total" > "start" + "limit". Both
//     "start" and "limit" come from the standard include_totals envelope; without
//     "limit" and "total" there is no reliable offset signal, so no hint is given.
//
// A bare array or any body without these fields yields no hint, because there is
// then no reliable signal that the result set was truncated. Only the scalar
// pagination fields are decoded (into a narrow struct) so a large result array is
// never allocated just to read them; json.Number preserves the original literals.
func paginationHint(rawJSON []byte) string {
	var envelope struct {
		Start  *json.Number `json:"start"`
		Limit  *json.Number `json:"limit"`
		Length *json.Number `json:"length"`
		Total  *json.Number `json:"total"`
		Next   *string      `json:"next"`
	}
	if err := json.Unmarshal(rawJSON, &envelope); err != nil {
		// A bare array or any non-object body carries no pagination metadata.
		return ""
	}

	length, hasLength := jsonNumberInt(envelope.Length)

	// Checkpoint pagination: mirror management.List.HasNext() (Next != ""), but
	// never point onward from a page that came back empty.
	if envelope.Next != nil && *envelope.Next != "" {
		if hasLength && length == 0 {
			return ""
		}
		return "This is one page of a checkpoint-paginated result set. There may be " +
			"more results; pass \"from\" set to the response's \"next\" token (and " +
			"optionally \"take\") in --query to fetch the next page."
	}

	// Offset pagination: mirror management.List.HasNext() — more pages exist when
	// total > start + limit.
	total, hasTotal := jsonNumberInt(envelope.Total)
	limit, hasLimit := jsonNumberInt(envelope.Limit)
	if !hasTotal || !hasLimit {
		return ""
	}
	start, _ := jsonNumberInt(envelope.Start) // Absent "start" means offset 0.
	if start+limit >= total {
		return ""
	}

	// Report the count actually returned ("length") when the envelope carries it;
	// otherwise state only the total so the message never overstates the page size.
	if hasLength {
		return fmt.Sprintf(
			"Showing %d of %d results (page starts at %d). More results exist; "+
				"pass a higher \"page\" or \"per_page\" in --query to fetch the rest.",
			length, total, start,
		)
	}

	return fmt.Sprintf(
		"This is one page of %d total results (page starts at %d). More results exist; "+
			"pass a higher \"page\" or \"per_page\" in --query to fetch the rest.",
		total, start,
	)
}

// jsonNumberInt reports the integer value of a json.Number field when it is
// present and holds an integer, and false when the field is absent (nil) or not an
// integer number.
func jsonNumberInt(n *json.Number) (int, bool) {
	if n == nil {
		return 0, false
	}
	v, err := n.Int64()
	if err != nil {
		return 0, false
	}
	return int(v), true
}

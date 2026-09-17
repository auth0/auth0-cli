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

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, rawJSON, "", "  "); err != nil {
		return fmt.Errorf("failed to format response: %w", err)
	}
	cli.renderer.OutputPreformattedJSON(ansi.ColorizeJSON(prettyJSON.String()))
	return nil
}

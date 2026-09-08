package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// jsonQuerySpec describes a list operation driven by a --query JSON payload.
type jsonQuerySpec struct {
	Path      string // API path segments (e.g. "actions/actions").
	SchemaCmd string
}

// runJSONQuery executes a GET request against the Management API with query parameters parsed from queryJSON.
func runJSONQuery(cli *cli, cmd *cobra.Command, spec jsonQuerySpec, queryJSON string) error {
	var queryParams map[string]interface{}
	if err := json.Unmarshal([]byte(queryJSON), &queryParams); err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected query parameters.", spec.SchemaCmd)
		return fmt.Errorf("invalid --query value: must be a JSON object: %w", err)
	}

	u, err := url.Parse(cli.api.HTTPClient.URI(strings.Split(spec.Path, "/")...))
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}
	q := u.Query()
	for key, val := range queryParams {
		q.Set(key, fmt.Sprintf("%v", val))
	}
	u.RawQuery = q.Encode()

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
	cli.renderer.Output(ansi.ColorizeJSON(prettyJSON.String()))
	return nil
}

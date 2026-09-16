package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

// mockHTTPClientAPI is a minimal implementation of auth0.HTTPClientAPI for testing.
type mockHTTPClientAPI struct {
	baseURL string
}

func (m *mockHTTPClientAPI) NewRequest(ctx context.Context, method, uri string, payload interface{}, opts ...management.RequestOption) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, uri, nil)
}

func (m *mockHTTPClientAPI) Do(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func (m *mockHTTPClientAPI) Request(ctx context.Context, method, uri string, payload interface{}, opts ...management.RequestOption) error {
	return nil
}

func (m *mockHTTPClientAPI) URI(path ...string) string {
	return m.baseURL + "/api/v2/" + strings.Join(path, "/")
}

func TestRunJSONQuery_InvalidJSON(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		api: &auth0.API{},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, "not-valid-json")

	assert.ErrorContains(t, err, "invalid --query value: must be a JSON object")
}

func TestRunJSONQuery_Success(t *testing.T) {
	expected := map[string]interface{}{
		"actions": []interface{}{},
		"total":   float64(0),
	}
	responseBody, err := json.Marshal(expected)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "post-login", r.URL.Query().Get("triggerId"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	var resultBuf strings.Builder
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  &resultBuf,
		},
		api: &auth0.API{
			HTTPClient: &mockHTTPClientAPI{baseURL: server.URL},
		},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{"triggerId":"post-login"}`)

	assert.NoError(t, err)
	assert.Contains(t, resultBuf.String(), "actions")
}

func TestRunJSONQuery_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"statusCode":401,"error":"Unauthorized","message":"Invalid token"}`))
	}))
	defer server.Close()

	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		api: &auth0.API{
			HTTPClient: &mockHTTPClientAPI{baseURL: server.URL},
		},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{"triggerId":"post-login"}`)

	assert.Error(t, err)
}

// decodeQueryParams mirrors runJSONQuery's decode step (UseNumber) so a unit test
// feeds encodeQueryParams the same value types the real path does.
func decodeQueryParams(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()
	var m map[string]interface{}
	require.NoError(t, decoder.Decode(&m))
	return m
}

func TestEncodeQueryParams(t *testing.T) {
	t.Run("array of scalars becomes repeated params", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"fields":["a","b","c"]}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, q["fields"])
		assert.Equal(t, "fields=a&fields=b&fields=c", q.Encode())
	})

	t.Run("numbers keep their literal, no scientific notation", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"per_page":1000000,"page":0}`))
		require.NoError(t, err)
		assert.Equal(t, "1000000", q.Get("per_page"))
		assert.Equal(t, "0", q.Get("page"))
	})

	t.Run("booleans stringify", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"include_totals":true,"deployed":false}`))
		require.NoError(t, err)
		assert.Equal(t, "true", q.Get("include_totals"))
		assert.Equal(t, "false", q.Get("deployed"))
	})

	t.Run("nested object is rejected", func(t *testing.T) {
		_, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"filter":{"k":"v"}}`))
		require.Error(t, err)
		assert.ErrorContains(t, err, "filter")
		assert.ErrorContains(t, err, "scalar")
	})

	t.Run("array containing an object is rejected", func(t *testing.T) {
		_, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"q":[{"nested":true}]}`))
		require.Error(t, err)
		assert.ErrorContains(t, err, "q")
	})
}

func TestRunJSONQuery_ArrayParamsSendRepeated(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actions":[]}`))
	}))
	defer server.Close()

	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{"fields":["id","name"]}`)

	require.NoError(t, err)
	assert.Contains(t, capturedURL, "fields=id")
	assert.Contains(t, capturedURL, "fields=name")
}

func TestRunJSONQuery_NestedObjectIsError(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: "http://example.invalid"}},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{"filter":{"nested":"value"}}`)

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid --query value")
	assert.ErrorContains(t, err, "filter")
}

func TestRunJSONQuery_BuildsURLWithQueryParams(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actions":[]}`))
	}))
	defer server.Close()

	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
		api: &auth0.API{
			HTTPClient: &mockHTTPClientAPI{baseURL: server.URL},
		},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{"deployed":"true","per_page":"5"}`)

	assert.NoError(t, err)
	assert.Contains(t, capturedURL, "deployed=true")
	assert.Contains(t, capturedURL, "per_page=5")
}

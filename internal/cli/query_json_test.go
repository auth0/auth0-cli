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

	t.Run("a Lucene-style string round-trips intact", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"q":"identities.connection:\"conn\" AND email:a@b.com"}`))
		require.NoError(t, err)
		assert.Equal(t, `identities.connection:"conn" AND email:a@b.com`, q.Get("q"))
		// Encode() must escape the colon, quotes, and spaces so the value survives
		// the wire and decodes back to the original.
		decoded, err := url.ParseQuery(q.Encode())
		require.NoError(t, err)
		assert.Equal(t, `identities.connection:"conn" AND email:a@b.com`, decoded.Get("q"))
	})

	t.Run("a null value omits the parameter", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"page":null,"per_page":10}`))
		require.NoError(t, err)
		_, hasPage := q["page"]
		assert.False(t, hasPage, "null value should be omitted, not sent as an empty param")
		assert.Equal(t, "10", q.Get("per_page"))
		assert.Equal(t, "per_page=10", q.Encode())
	})

	t.Run("an empty string is kept as an empty-valued param", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"q":""}`))
		require.NoError(t, err)
		values, hasQ := q["q"]
		assert.True(t, hasQ, "an explicit empty string is a value, unlike null")
		assert.Equal(t, []string{""}, values)
	})

	t.Run("an empty array produces no param", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"fields":[]}`))
		require.NoError(t, err)
		_, hasFields := q["fields"]
		assert.False(t, hasFields)
		assert.Empty(t, q.Encode())
	})

	t.Run("null elements in an array are skipped", func(t *testing.T) {
		q, err := encodeQueryParams(url.Values{}, decodeQueryParams(t, `{"fields":["a",null,"b"]}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, q["fields"])
	})

	t.Run("a float64 number formats without scientific notation", func(t *testing.T) {
		// A caller that decodes without UseNumber yields float64 rather than
		// json.Number; encodeQueryParams must still format the literal.
		q, err := encodeQueryParams(url.Values{}, map[string]interface{}{"per_page": float64(1000000)})
		require.NoError(t, err)
		assert.Equal(t, "1000000", q.Get("per_page"))
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

// TestRunJSONQuery_InvalidQueryClassifiesAsValidation covers that client-side
// --query failures classify as "validation" (not "unknown") in the error
// envelope, matching how a server-side 400/422 and the --data path classify.
func TestRunJSONQuery_InvalidQueryClassifiesAsValidation(t *testing.T) {
	newCLI := func() *cli {
		return &cli{
			renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
			api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: "http://example.invalid"}},
		}
	}

	t.Run("malformed JSON", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runJSONQuery(newCLI(), cmd, jsonQuerySpec{
			Path:      "actions/actions",
			SchemaCmd: "auth0 actions list",
		}, `{not json`)
		require.Error(t, err)
		assert.Equal(t, "validation", errorClass(err))
	})

	t.Run("nested object", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runJSONQuery(newCLI(), cmd, jsonQuerySpec{
			Path:      "actions/actions",
			SchemaCmd: "auth0 actions list",
		}, `{"filter":{"nested":"value"}}`)
		require.Error(t, err)
		assert.Equal(t, "validation", errorClass(err))
	})
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

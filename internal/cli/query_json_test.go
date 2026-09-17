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

func TestRunJSONQuery_CompactOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actions":[{"id":"a"}],"total":1}`))
	}))
	defer server.Close()

	var resultBuf strings.Builder
	cli := &cli{
		jsonCompact: true,
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  &resultBuf,
		},
		api: &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "actions/actions",
		SchemaCmd: "auth0 actions list",
	}, `{}`)

	require.NoError(t, err)
	out := strings.TrimSpace(resultBuf.String())
	// Compact output is a single dense line with no indentation newlines.
	assert.Equal(t, `{"actions":[{"id":"a"}],"total":1}`, out)
	assert.NotContains(t, out, "\n")
}

func TestRunJSONQuery_CSVIsRejected(t *testing.T) {
	cli := &cli{
		csv: true,
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
	}, `{}`)

	require.Error(t, err)
	assert.ErrorContains(t, err, "--csv is not supported with --query")
}

func TestPaginationHint(t *testing.T) {
	t.Run("offset with totals, more results signals a hint", func(t *testing.T) {
		// Standard include_totals envelope: first of two pages.
		hint := paginationHint([]byte(`{"clients":[],"total":100,"start":0,"limit":50,"length":50}`))
		assert.Contains(t, hint, "Showing 50 of 100")
		assert.Contains(t, hint, "More results exist")
	})

	t.Run("offset uses length for the returned count", func(t *testing.T) {
		// "length" is the number actually returned; the hint reports it, not "limit".
		hint := paginationHint([]byte(`{"clients":[],"total":100,"start":0,"limit":50,"length":42}`))
		assert.Contains(t, hint, "Showing 42 of 100")
	})

	t.Run("offset without limit yields no hint (spurious total is ignored)", func(t *testing.T) {
		// A "total" without the rest of the include_totals envelope is not a
		// reliable pagination signal, so no hint is emitted.
		hint := paginationHint([]byte(`{"actions":[{"id":"a"},{"id":"b"}],"total":10}`))
		assert.Empty(t, hint)
	})

	t.Run("checkpoint pagination signals a hint", func(t *testing.T) {
		hint := paginationHint([]byte(`{"logs":[{"id":"a"}],"next":"tok_abc"}`))
		assert.Contains(t, hint, "checkpoint")
		assert.Contains(t, hint, "next")
		// Wording stays tentative: a final page can still carry a "next" token.
		assert.Contains(t, hint, "There may be")
		assert.NotContains(t, hint, "More results exist")
	})

	t.Run("offset without length states the total but no page count", func(t *testing.T) {
		// No "length" field: the hint must not invent a returned count from "limit".
		hint := paginationHint([]byte(`{"clients":[{"id":"a"}],"total":100,"start":0,"limit":50}`))
		assert.Contains(t, hint, "one page of 100 total results")
		assert.Contains(t, hint, "More results exist")
		assert.NotContains(t, hint, "Showing")
	})

	t.Run("checkpoint with an empty page has no hint", func(t *testing.T) {
		// A "next" token on a page that returned nothing must not claim more results.
		hint := paginationHint([]byte(`{"users":[],"next":"tok_abc","length":0}`))
		assert.Empty(t, hint)
	})

	t.Run("complete result set has no hint", func(t *testing.T) {
		hint := paginationHint([]byte(`{"clients":[{"id":"a"},{"id":"b"}],"total":2,"start":0,"limit":50,"length":2}`))
		assert.Empty(t, hint)
	})

	t.Run("last page has no hint", func(t *testing.T) {
		hint := paginationHint([]byte(`{"clients":[{"id":"a"}],"total":100,"start":99,"limit":50,"length":1}`))
		assert.Empty(t, hint)
	})

	t.Run("empty checkpoint token has no hint", func(t *testing.T) {
		hint := paginationHint([]byte(`{"logs":[{"id":"a"}],"next":""}`))
		assert.Empty(t, hint)
	})

	t.Run("no total and no next has no hint", func(t *testing.T) {
		hint := paginationHint([]byte(`{"actions":[{"id":"a"}]}`))
		assert.Empty(t, hint)
	})

	t.Run("bare array has no hint", func(t *testing.T) {
		hint := paginationHint([]byte(`[{"id":"a"}]`))
		assert.Empty(t, hint)
	})
}

func TestRunJSONQuery_TruncationWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clients":[{"id":"a"},{"id":"b"}],"total":100,"start":0,"limit":50}`))
	}))
	defer server.Close()

	var msgBuf strings.Builder
	cli := &cli{
		renderer: &display.Renderer{
			MessageWriter: &msgBuf,
			ResultWriter:  io.Discard,
		},
		api: &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runJSONQuery(cli, cmd, jsonQuerySpec{
		Path:      "clients",
		SchemaCmd: "auth0 apps list",
	}, `{}`)

	require.NoError(t, err)
	assert.Contains(t, msgBuf.String(), "More results exist")
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

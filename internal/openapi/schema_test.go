package openapi

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalOpenAPIDoc = `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{}}`

func TestFetchDocRetriesTransientFailures(t *testing.T) {
	t.Run("succeeds after transient failures", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Fail with a 404 for the first two attempts, then succeed, mirroring
			// the flaky edge-cache behavior the retry is meant to absorb.
			if atomic.AddInt32(&calls, 1) < 3 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(minimalOpenAPIDoc))
		}))
		defer srv.Close()

		defer withSchemaTestOverrides(srv.URL)()

		doc, err := fetchDoc()
		require.NoError(t, err)
		require.NotNil(t, doc)
		assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
	})

	t.Run("returns error after exhausting all attempts", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		defer withSchemaTestOverrides(srv.URL)()

		doc, err := fetchDoc()
		require.Error(t, err)
		assert.Nil(t, doc)
		assert.Contains(t, err.Error(), "unexpected status code: 404")
		assert.Equal(t, int32(schemaFetchAttempts), atomic.LoadInt32(&calls))
	})
}

// withSchemaTestOverrides points the fetcher at url and shrinks the backoff so the
// retry test does not sleep for real. It returns a func that restores the globals.
func withSchemaTestOverrides(url string) func() {
	prevEndpoint := schemaEndpoint
	prevBackoff := schemaFetchBackoff
	schemaEndpoint = url
	schemaFetchBackoff = time.Millisecond
	return func() {
		schemaEndpoint = prevEndpoint
		schemaFetchBackoff = prevBackoff
	}
}

func TestGetDoc(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.NotEmpty(t, doc.OpenAPI)
	assert.NotNil(t, doc.Paths)
	assert.NotNil(t, doc.Components)
	assert.NotNil(t, doc.Components.Schemas)
}

func TestFindOperation(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	tests := []struct {
		name              string
		method            string
		path              string
		expectError       bool
		expectOperationID string
	}{
		{
			name:              "POST actions/actions",
			method:            "POST",
			path:              "/actions/actions",
			expectError:       false,
			expectOperationID: "post_action",
		},
		{
			name:              "GET actions/actions",
			method:            "GET",
			path:              "/actions/actions",
			expectError:       false,
			expectOperationID: "get_actions",
		},
		{
			name:        "Invalid path",
			method:      "GET",
			path:        "/invalid/path",
			expectError: true,
		},
		{
			name:        "Invalid method",
			method:      "INVALID",
			path:        "/actions/actions",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation, err := FindOperation(doc, tt.method, tt.path)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, operation)
			} else {
				require.NoError(t, err)
				require.NotNil(t, operation)
				assert.Equal(t, tt.expectOperationID, operation.OperationID)
			}
		})
	}
}

func TestGetRequestSchema(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	operation, err := FindOperation(doc, "POST", "/actions/actions")
	require.NoError(t, err)

	requestSchema := GetRequestSchema(operation)
	require.NotNil(t, requestSchema)
	require.NotNil(t, requestSchema.Value)

	// Verify it has the expected required fields.
	assert.Contains(t, requestSchema.Value.Required, "name")
	assert.Contains(t, requestSchema.Value.Required, "supported_triggers")
}

func TestGetQueryParamSchema(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	t.Run("returns schema for operation with query params", func(t *testing.T) {
		operation, err := FindOperation(doc, "GET", "/actions/actions")
		require.NoError(t, err)

		schema := GetQueryParamSchema(operation)
		require.NotNil(t, schema)
		assert.NotEmpty(t, schema.Properties)
		assert.True(t, schema.Type.Is("object"))
	})

	t.Run("returns nil when no query params exist", func(t *testing.T) {
		// POST /actions/actions has no query params.
		operation, err := FindOperation(doc, "POST", "/actions/actions")
		require.NoError(t, err)

		schema := GetQueryParamSchema(operation)
		assert.Nil(t, schema)
	})

	t.Run("includes only query params, not path or header params", func(t *testing.T) {
		operation, err := FindOperation(doc, "GET", "/actions/actions")
		require.NoError(t, err)

		schema := GetQueryParamSchema(operation)
		require.NotNil(t, schema)

		for name := range schema.Properties {
			// Path-level params (like action id) should not appear.
			assert.NotEqual(t, "id", name)
		}
	})
}

func TestExtractPathFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Full URL with tenant",
			url:      "https://tenant.auth0.com/api/v2/actions/actions",
			expected: "/actions/actions",
		},
		{
			name:     "URL with path parameters",
			url:      "https://tenant.auth0.com/api/v2/actions/actions/act_123",
			expected: "/actions/actions/act_123",
		},
		{
			name:     "URL without api/v2",
			url:      "https://tenant.auth0.com/some/path",
			expected: "",
		},
		{
			name:     "URL with query parameters",
			url:      "https://tenant.auth0.com/api/v2/actions/actions?page=1",
			expected: "/actions/actions?page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPathFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCaching(t *testing.T) {
	// First call - should fetch or load from cache.
	doc1, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc1)

	// Second call - should return cached doc.
	doc2, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc2)

	// Should be the same instance (pointer equality).
	assert.Equal(t, doc1, doc2)
}

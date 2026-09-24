package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
)

// buildDocsSearchServer creates an httptest server that responds with the given
// status code and body. The handler also captures the decoded request body into
// capturedBody if non-nil.
func buildDocsSearchServer(t *testing.T, status int, body string, capturedBody *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		if capturedBody != nil {
			var m map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&m); err == nil {
				*capturedBody = m
			}
		}

		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
}

// overrideDocsSearchURL replaces the package-level docsSearchURL with the given
// value and restores the original via t.Cleanup.
func overrideDocsSearchURL(t *testing.T, url string) {
	t.Helper()
	old := docsSearchURL
	docsSearchURL = url
	t.Cleanup(func() { docsSearchURL = old })
}

// TestRunDocsSearch_HappyPath verifies that runDocsSearch parses results
// correctly and returns them sorted by descending score.
func TestRunDocsSearch_HappyPath(t *testing.T) {
	// Return two results intentionally out of order: first has score 1.0,
	// second has score 9.0. After the call the slice must be [9.0, 1.0].
	responseBody := `{
		"results": [
			{
				"page": "docs/get-started",
				"header": "Get Started",
				"content": "Get Started\nLearn how to integrate Auth0.",
				"metadata": {
					"title": "Auth0 Get Started",
					"breadcrumbs": ["Docs", "Get Started"],
					"hash": "intro",
					"openapi": ""
				},
				"score": 1.0
			},
			{
				"page": "docs/quickstart",
				"header": "Quickstart",
				"content": "Quickstart\nRun through a quick example.",
				"metadata": {
					"title": "Auth0 Quickstart",
					"breadcrumbs": ["Docs", "Quickstart"],
					"hash": "step1",
					"openapi": ""
				},
				"score": 9.0
			}
		]
	}`

	server := buildDocsSearchServer(t, http.StatusOK, responseBody, nil)
	defer server.Close()
	overrideDocsSearchURL(t, server.URL)

	results, err := runDocsSearch(context.Background(), "get started", "en")
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Sorted descending: 9.0 first, 1.0 second.
	assert.Equal(t, float64(9.0), results[0].Score)
	assert.Equal(t, float64(1.0), results[1].Score)

	// Field parsing for the top result.
	assert.Equal(t, "docs/quickstart", results[0].Page)
	assert.Equal(t, "Quickstart", results[0].Header)
	assert.Equal(t, "Auth0 Quickstart", results[0].Metadata.Title)
	assert.Equal(t, []string{"Docs", "Quickstart"}, results[0].Metadata.Breadcrumbs)
	assert.Equal(t, "step1", results[0].Metadata.Hash)
	assert.Equal(t, "", results[0].Metadata.OpenAPI)
}

// TestRunDocsSearch_RequestShape verifies the HTTP request shape sent by
// runDocsSearch, including method, Content-Type, and body payload.
func TestRunDocsSearch_RequestShape(t *testing.T) {
	t.Run("explicit language is forwarded in filters", func(t *testing.T) {
		var captured map[string]interface{}
		server := buildDocsSearchServer(t, http.StatusOK, `{"results":[]}`, &captured)
		defer server.Close()
		overrideDocsSearchURL(t, server.URL)

		_, err := runDocsSearch(context.Background(), "mfa", "ja")
		require.NoError(t, err)

		require.NotNil(t, captured)
		assert.Equal(t, "mfa", captured["query"])

		filters, ok := captured["filters"].(map[string]interface{})
		require.True(t, ok, "filters should be a map")
		assert.Equal(t, "ja", filters["language"])
	})

	t.Run("language is normalized before forwarding", func(t *testing.T) {
		var captured map[string]interface{}
		server := buildDocsSearchServer(t, http.StatusOK, `{"results":[]}`, &captured)
		defer server.Close()
		overrideDocsSearchURL(t, server.URL)

		_, err := runDocsSearch(context.Background(), "mfa", "  FR ")
		require.NoError(t, err)

		require.NotNil(t, captured)
		filters, ok := captured["filters"].(map[string]interface{})
		require.True(t, ok, "filters should be a map")
		assert.Equal(t, "fr", filters["language"])
	})

	t.Run("empty language defaults to en", func(t *testing.T) {
		var captured map[string]interface{}
		server := buildDocsSearchServer(t, http.StatusOK, `{"results":[]}`, &captured)
		defer server.Close()
		overrideDocsSearchURL(t, server.URL)

		_, err := runDocsSearch(context.Background(), "x", "")
		require.NoError(t, err)

		require.NotNil(t, captured)
		filters, ok := captured["filters"].(map[string]interface{})
		require.True(t, ok, "filters should be a map")
		assert.Equal(t, "en", filters["language"])
	})
}

// TestRunDocsSearch_UnsupportedLanguage verifies that a language the docs index is
// not localized in is rejected before any network call, with a helpful message.
func TestRunDocsSearch_UnsupportedLanguage(t *testing.T) {
	// Point the URL at a server that would fail the test if it were ever hit, to
	// prove validation happens before the request.
	overrideDocsSearchURL(t, "http://127.0.0.1:0")

	_, err := runDocsSearch(context.Background(), "mfa", "es")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"es"`)
	assert.Contains(t, err.Error(), "en, fr, ja")
}

// TestValidateDocsLanguage covers the supported/unsupported language check.
func TestValidateDocsLanguage(t *testing.T) {
	for _, lang := range []string{"en", "fr", "ja"} {
		assert.NoError(t, validateDocsLanguage(lang), "%q should be supported", lang)
	}
	for _, lang := range []string{"es", "de", "pt", "zh", "xx", ""} {
		assert.Error(t, validateDocsLanguage(lang), "%q should be rejected", lang)
	}
}

// TestRunDocsSearch_Non200 verifies that a non-200 HTTP status is surfaced as
// an error whose message includes the status code.
func TestRunDocsSearch_Non200(t *testing.T) {
	server := buildDocsSearchServer(t, http.StatusInternalServerError, `internal error`, nil)
	defer server.Close()
	overrideDocsSearchURL(t, server.URL)

	_, err := runDocsSearch(context.Background(), "anything", "en")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// TestRunDocsSearch_InvalidJSON verifies that a 200 response with an
// unparseable body is returned as a parse error.
func TestRunDocsSearch_InvalidJSON(t *testing.T) {
	server := buildDocsSearchServer(t, http.StatusOK, "not json", nil)
	defer server.Close()
	overrideDocsSearchURL(t, server.URL)

	_, err := runDocsSearch(context.Background(), "anything", "en")
	require.Error(t, err)
}

// TestRunDocsSearch_EmptyResults verifies that an empty results array is
// returned without error.
func TestRunDocsSearch_EmptyResults(t *testing.T) {
	server := buildDocsSearchServer(t, http.StatusOK, `{"results":[]}`, nil)
	defer server.Close()
	overrideDocsSearchURL(t, server.URL)

	results, err := runDocsSearch(context.Background(), "anything", "en")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// TestSearchDocsCmd_OpenRejectedInAgentMode verifies that --open fails fast in agent
// mode instead of guessing at the top result, since an agent cannot open a browser or
// pick a result and already gets each result's raw-markdown URL in the JSON output.
func TestSearchDocsCmd_OpenRejectedInAgentMode(t *testing.T) {
	c := &cli{agentMode: true, renderer: &display.Renderer{}}
	cmd := searchDocsCmd(c)
	cmd.SetArgs([]string{"actions", "--open"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported in agent mode")
}

// TestDocsResultURL_UsesBaseURL verifies docsResultURL builds off the docsBaseURL
// package var so tests (and the markdown fetch) can be pointed at a test server.
func TestDocsResultURL_UsesBaseURL(t *testing.T) {
	old := docsBaseURL
	docsBaseURL = "https://example.test/"
	t.Cleanup(func() { docsBaseURL = old })

	r := docsSearchResult{Page: "docs/x"}
	assert.Equal(t, "https://example.test/docs/x", docsResultURL(r, false))
	assert.Equal(t, "https://example.test/docs/x.md", docsResultURL(r, true))
}

// TestDocsResultURL exercises the URL-building logic for agent mode and human mode.
func TestDocsResultURL(t *testing.T) {
	tests := []struct {
		name      string
		page      string
		hash      string
		agentMode bool
		expected  string
	}{
		{
			name:      "agent mode adds .md suffix and omits hash",
			page:      "docs/security/mfa",
			hash:      "overview",
			agentMode: true,
			expected:  "https://auth0.com/docs/security/mfa.md",
		},
		{
			name:      "human mode without hash returns bare URL",
			page:      "docs/quickstart",
			hash:      "",
			agentMode: false,
			expected:  "https://auth0.com/docs/quickstart",
		},
		{
			name:      "human mode with hash appends anchor",
			page:      "docs/quickstart",
			hash:      "sec",
			agentMode: false,
			expected:  "https://auth0.com/docs/quickstart#sec",
		},
		{
			name:      "leading-slash page does not produce a double slash",
			page:      "/docs/quickstart",
			hash:      "",
			agentMode: false,
			expected:  "https://auth0.com/docs/quickstart",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := docsSearchResult{Page: tc.page}
			r.Metadata.Hash = tc.hash
			assert.Equal(t, tc.expected, docsResultURL(r, tc.agentMode))
		})
	}
}

// TestDocsShouldPromptForResult verifies the interactive picker gate, in particular
// that a machine output format suppresses the picker even in an interactive terminal
// with multiple results (so --json --open does not launch a prompt over the JSON).
func TestDocsShouldPromptForResult(t *testing.T) {
	tests := []struct {
		name          string
		interactive   bool
		resultCount   int
		machineFormat bool
		expected      bool
	}{
		{
			name:        "interactive TTY with multiple results prompts",
			interactive: true,
			resultCount: 3,
			expected:    true,
		},
		{
			name:          "machine format suppresses the picker even in a TTY",
			interactive:   true,
			resultCount:   3,
			machineFormat: true,
			expected:      false,
		},
		{
			name:        "single result never prompts",
			interactive: true,
			resultCount: 1,
			expected:    false,
		},
		{
			name:        "non-interactive never prompts",
			interactive: false,
			resultCount: 3,
			expected:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, docsShouldPromptForResult(tc.interactive, tc.resultCount, tc.machineFormat))
		})
	}
}

// TestDocsResultType verifies that prose pages return "Doc" and API reference
// pages return "<METHOD> <PATH>".
func TestDocsResultType(t *testing.T) {
	tests := []struct {
		name     string
		openapi  string
		expected string
	}{
		{
			name:     "empty openapi returns Doc",
			openapi:  "",
			expected: "Doc",
		},
		{
			name:     "POST endpoint",
			openapi:  "docs/oas/management.json POST /users",
			expected: "POST /users",
		},
		{
			name:     "DELETE endpoint with path param",
			openapi:  "docs/oas/management.json DELETE /roles/{id}",
			expected: "DELETE /roles/{id}",
		},
		{
			name:     "single-token openapi returns the token verbatim",
			openapi:  "POST",
			expected: "POST",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := docsSearchResult{}
			r.Metadata.OpenAPI = tc.openapi
			assert.Equal(t, tc.expected, docsResultType(r))
		})
	}
}

// TestDocsResultTitle verifies that the metadata title is preferred over the
// header and that the header is used as a fallback when title is empty.
func TestDocsResultTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		header   string
		expected string
	}{
		{
			name:     "metadata title is returned when set",
			title:    "Authentication Flows",
			header:   "Auth Flows",
			expected: "Authentication Flows",
		},
		{
			name:     "falls back to header when title is empty",
			title:    "",
			header:   "Auth Flows",
			expected: "Auth Flows",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := docsSearchResult{Header: tc.header}
			r.Metadata.Title = tc.title
			assert.Equal(t, tc.expected, docsResultTitle(r))
		})
	}
}

// TestDocsResultSection verifies that breadcrumbs are joined with " > " and
// that an empty slice yields an empty string.
func TestDocsResultSection(t *testing.T) {
	tests := []struct {
		name        string
		breadcrumbs []string
		expected    string
	}{
		{
			name:        "multiple breadcrumbs joined with >",
			breadcrumbs: []string{"A", "B", "C"},
			expected:    "A > B > C",
		},
		{
			name:        "empty breadcrumbs returns empty string",
			breadcrumbs: []string{},
			expected:    "",
		},
		{
			name:        "nil breadcrumbs returns empty string",
			breadcrumbs: nil,
			expected:    "",
		},
		{
			name:        "single breadcrumb returned as-is",
			breadcrumbs: []string{"Docs"},
			expected:    "Docs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := docsSearchResult{}
			r.Metadata.Breadcrumbs = tc.breadcrumbs
			assert.Equal(t, tc.expected, docsResultSection(r))
		})
	}
}

// TestDocsSnippet exercises the snippet-extraction logic: header stripping,
// whitespace collapsing, and rune-safe truncation.
func TestDocsSnippet(t *testing.T) {
	t.Run("strips leading header and collapses whitespace", func(t *testing.T) {
		r := docsSearchResult{
			Header:  "Foo",
			Content: "Foo\nBar baz  qux",
		}
		assert.Equal(t, "Bar baz qux", docsSnippet(r))
	})

	t.Run("short content is returned unchanged (no ellipsis)", func(t *testing.T) {
		r := docsSearchResult{
			Header:  "Title",
			Content: "Title\nShort description.",
		}
		result := docsSnippet(r)
		assert.Equal(t, "Short description.", result)
		assert.False(t, strings.HasSuffix(result, "..."), "short content should not get ellipsis")
	})

	t.Run("content longer than 160 runes is truncated with ellipsis", func(t *testing.T) {
		// Build a string that is definitely longer than 160 runes.
		longContent := strings.Repeat("word ", 50) // 250 runes.
		r := docsSearchResult{
			Header:  "",
			Content: longContent,
		}
		result := docsSnippet(r)
		assert.True(t, strings.HasSuffix(result, "..."), "long content should end with ...")
		// Result length: at most 160 rune content + 3 "..." chars.
		runes := []rune(result)
		assert.LessOrEqual(t, len(runes), 163)
	})

	t.Run("multi-byte (French) long content truncation produces valid UTF-8", func(t *testing.T) {
		// Build a long string with multi-byte runes so a naive byte-level
		// truncation would split a character.
		longContent := strings.Repeat("é", 200) // Each 'é' is 2 bytes, 200 runes total.
		r := docsSearchResult{
			Header:  "",
			Content: longContent,
		}
		result := docsSnippet(r)
		assert.True(t, utf8.ValidString(result), "result must be valid UTF-8 after truncation")
		assert.True(t, strings.HasSuffix(result, "..."), "long content should end with ...")
		runes := []rune(result)
		assert.LessOrEqual(t, len(runes), 163)
	})

	t.Run("empty content returns empty string", func(t *testing.T) {
		r := docsSearchResult{Header: "H", Content: ""}
		assert.Equal(t, "", docsSnippet(r))
	})

	t.Run("content that is only the header returns empty string", func(t *testing.T) {
		r := docsSearchResult{Header: "Header", Content: "Header"}
		assert.Equal(t, "", docsSnippet(r))
	})
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// docsSearchURL is the public docs search index; a package var so tests can
// point it at an httptest server.
var docsSearchURL = "https://leaves.mintlify.com/api/search/auth0"

// docsBaseURL is the public docs host; a package var so tests can point the
// page/markdown URLs at an httptest server.
var docsBaseURL = "https://auth0.com/"

// docsSupportedLanguages are the languages the Auth0 docs index is actually
// localized in. Any other value makes the API silently fall back to a single
// irrelevant result, so we reject unsupported values up front instead.
var docsSupportedLanguages = []string{"en", "fr", "ja"}

const docsSnippetMaxLen = 160

type docsSearchResult struct {
	Page     string `json:"page"`
	Header   string `json:"header"`
	Content  string `json:"content"`
	Metadata struct {
		Title       string   `json:"title"`
		Breadcrumbs []string `json:"breadcrumbs"`
		Hash        string   `json:"hash"`
		OpenAPI     string   `json:"openapi"`
	} `json:"metadata"`
	Score float64 `json:"score"`
}

type docsSearchResponse struct {
	Results []docsSearchResult `json:"results"`
}

var docsSearchQuery = Argument{
	Name: "Query",
	Help: "Search term to look up in the Auth0 documentation.",
}

var (
	docsSearchLanguage = Flag{
		Name:      "Language",
		LongForm:  "language",
		ShortForm: "l",
		Help:      "Documentation language to search. One of: en, fr, ja.",
	}
	docsSearchOpen = Flag{
		Name:     "Open",
		LongForm: "open",
		Help:     "Open a result in the browser. In an interactive terminal you pick which one; otherwise the top result opens. In agent mode no browser opens; the top result's markdown content is printed instead.",
	}
)

func docsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Search the Auth0 documentation",
		Long:  "Search the official Auth0 documentation from your terminal.",
	}
	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(searchDocsCmd(cli))
	return cmd
}

func searchDocsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Query    string
		Language string
		Open     bool
	}

	cmd := &cobra.Command{
		Use:     "search",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"find"},
		Short:   "Search the Auth0 documentation",
		Long:    "Search the official Auth0 documentation and print matching pages with their URLs.",
		Example: `  auth0 docs search browser
  auth0 docs search "custom domains"
  auth0 docs search "refresh token" --json
  auth0 docs search mfa --language ja
  auth0 docs search actions --open
  auth0 docs search rules --json-compact | jq '.[] | {title, url}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.Query = args[0]
			}
			if inputs.Query == "" {
				if err := docsSearchQuery.Ask(cmd, &inputs.Query); err != nil {
					return err
				}
			}

			var results []docsSearchResult
			if err := ansi.Waiting(func() error {
				var err error
				results, err = runDocsSearch(cmd.Context(), inputs.Query, inputs.Language)
				return err
			}); err != nil {
				return err
			}

			// Agent mode --open: an agent has no browser, so fetch the top result's
			// raw markdown and emit it as JSON instead of launching one.
			if inputs.Open && cli.agentMode {
				if len(results) == 0 {
					cli.renderer.DocsSearchResults(nil, inputs.Query)
					return nil
				}
				return fetchDocsMarkdown(cmd, cli, results[0])
			}

			views := make([]display.DocsSearchResult, 0, len(results))
			for _, r := range results {
				views = append(views, display.DocsSearchResult{
					Title:   docsResultTitle(r),
					Section: docsResultSection(r),
					Type:    docsResultType(r),
					URL:     docsResultURL(r, cli.agentMode),
					Snippet: docsSnippet(r),
					Score:   r.Score,
				})
			}

			// A machine output format (--json/--json-compact/--csv) signals scripting
			// intent, so keep that output intact and open the top result without a
			// picker. The interactive picker is only for the default human format,
			// where it also lists the results, so we skip the table to avoid showing
			// them twice.
			machineFormat := cli.json || cli.jsonCompact || cli.csv
			interactivePick := inputs.Open && canPrompt(cmd) && len(views) > 1 && !machineFormat
			if !interactivePick {
				cli.renderer.DocsSearchResults(views, inputs.Query)
			}

			if inputs.Open && len(views) > 0 {
				return openDocsResult(cmd, cli, results)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	docsSearchLanguage.RegisterString(cmd, &inputs.Language, "en")
	docsSearchOpen.RegisterBool(cmd, &inputs.Open, false)

	return cmd
}

// runDocsSearch calls the public Auth0 docs search index and returns results sorted by
// descending relevance score (the API returns them unsorted). The API caps results at ~6.
func runDocsSearch(ctx context.Context, query, language string) ([]docsSearchResult, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		language = "en"
	}
	if err := validateDocsLanguage(language); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"query":   query,
		"filters": map[string]string{"language": language},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, docsSearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://auth0.com")
	req.Header.Set("Referer", "https://auth0.com/")
	req.Header.Set("User-Agent", "auth0-cli")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach the Auth0 documentation search service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("documentation search failed with status code %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var parsed docsSearchResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	sort.SliceStable(parsed.Results, func(i, j int) bool {
		return parsed.Results[i].Score > parsed.Results[j].Score
	})
	return parsed.Results, nil
}

// validateDocsLanguage rejects languages the docs index is not localized in, since
// the API silently returns a single irrelevant result for those rather than erroring.
func validateDocsLanguage(language string) error {
	for _, l := range docsSupportedLanguages {
		if language == l {
			return nil
		}
	}
	return fmt.Errorf("unsupported documentation language %q (supported: %s)", language, strings.Join(docsSupportedLanguages, ", "))
}

// docsResultURL builds the public URL. Agent mode gets the raw-markdown ".md" form
// (no anchor); human mode gets the normal page plus a "#hash" anchor when present.
func docsResultURL(r docsSearchResult, agentMode bool) string {
	url := docsBaseURL + r.Page
	if agentMode {
		return url + ".md"
	}
	if r.Metadata.Hash != "" {
		url += "#" + r.Metadata.Hash
	}
	return url
}

// docsResultType returns "Doc" for prose pages, or the "<METHOD> <PATH>" parsed from the
// openapi metadata for Management API reference pages (e.g. "POST /users").
func docsResultType(r docsSearchResult) string {
	fields := strings.Fields(r.Metadata.OpenAPI)
	if len(fields) >= 2 {
		return fields[len(fields)-2] + " " + fields[len(fields)-1]
	}
	return "Doc"
}

func docsResultTitle(r docsSearchResult) string {
	if r.Metadata.Title != "" {
		return r.Metadata.Title
	}
	return r.Header
}

func docsResultSection(r docsSearchResult) string {
	return strings.Join(r.Metadata.Breadcrumbs, " > ")
}

// docsSnippet strips the leading duplicated header line from content, collapses
// whitespace, and truncates to docsSnippetMaxLen with an ellipsis.
func docsSnippet(r docsSearchResult) string {
	content := strings.TrimSpace(r.Content)
	content = strings.TrimPrefix(content, r.Header)
	content = strings.Join(strings.Fields(content), " ")
	// Truncate on rune boundaries so multi-byte characters in non-English docs
	// (selected via --language) are never split into an invalid byte sequence.
	if runes := []rune(content); len(runes) > docsSnippetMaxLen {
		content = strings.TrimSpace(string(runes[:docsSnippetMaxLen])) + "..."
	}
	return content
}

// openDocsResult opens a result in the browser: interactive pick in a TTY, top result
// otherwise. It always opens the human-facing page (never the ".md" raw-markdown form),
// since opening a browser is inherently an interactive, human action.
func openDocsResult(cmd *cobra.Command, cli *cli, results []docsSearchResult) error {
	target := docsResultURL(results[0], false)
	if canPrompt(cmd) && len(results) > 1 {
		labels := make([]string, len(results))
		byLabel := make(map[string]string, len(results))
		for i, r := range results {
			// Label as "Title (URL)"; the URL keeps labels unique when two pages
			// share a title.
			label := fmt.Sprintf("%s (%s)", docsResultTitle(r), docsResultURL(r, false))
			labels[i] = label
			byLabel[label] = docsResultURL(r, false)
		}
		var choice string
		q := prompt.SelectInput("result", "Select a result to open:", "Choose which documentation page to open in your browser.", labels, labels[0], true)
		if err := prompt.AskOne(q, &choice); err != nil {
			return err
		}
		target = byLabel[choice]
	}
	cli.renderer.Infof("Opening %s", target)
	if err := browser.OpenURL(target); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	return nil
}

// docsMarkdownOutput is the JSON shape emitted by agent-mode --open: the top result's
// title, its ".md" URL, and the fetched raw markdown content.
type docsMarkdownOutput struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// fetchDocsMarkdown fetches a result's raw markdown (the ".md" URL) and emits it as JSON.
// It backs agent-mode --open, where launching a browser makes no sense, so the agent gets
// the documentation text directly on stdout.
func fetchDocsMarkdown(cmd *cobra.Command, cli *cli, result docsSearchResult) error {
	url := docsResultURL(result, true)

	var content string
	if err := ansi.Waiting(func() error {
		var err error
		content, err = fetchDocsMarkdownContent(cmd.Context(), url)
		return err
	}); err != nil {
		return err
	}

	out := docsMarkdownOutput{
		Title:   docsResultTitle(result),
		URL:     url,
		Content: content,
	}
	if cli.jsonCompact {
		cli.renderer.JSONCompactResult(out)
	} else {
		cli.renderer.JSONResult(out)
	}
	return nil
}

// fetchDocsMarkdownContent GETs the given ".md" URL and returns its raw body.
func fetchDocsMarkdownContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build documentation request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown, text/plain, */*")
	req.Header.Set("User-Agent", "auth0-cli")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch documentation content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching documentation content failed with status code %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read documentation content: %w", err)
	}
	// Return the markdown verbatim (it is already the ideal, structured form for an
	// agent); only trim surrounding whitespace so the content starts cleanly.
	return strings.TrimSpace(string(raw)), nil
}

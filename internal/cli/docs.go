package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// docsSearchResponseMaxBytes bounds the search response read so a malformed or
// oversized response cannot exhaust memory. The index returns ~6 small results.
const docsSearchResponseMaxBytes = 512 * 1024 // 512 KiB.

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
		Help:     "Open a result in the browser. In an interactive terminal you pick which one; otherwise the top result opens. Not supported in agent mode.",
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
			// --open opens a result in a browser and, in a terminal, lets a human
			// pick which one. An agent can do neither, and it already gets every
			// result's raw-markdown ".md" URL in the normal JSON output to fetch
			// directly, so fail fast rather than guessing at the top result.
			if inputs.Open && cli.agentMode {
				return usageError{
					err:    fmt.Errorf("`--open` opens a result in a browser and is not supported in agent mode; run the search without `--open` and fetch a result's `url` (raw markdown) directly"),
					reason: "unsupported_in_agent_mode",
				}
			}

			if len(args) > 0 {
				inputs.Query = args[0]
			}
			if inputs.Query == "" {
				if err := docsSearchQuery.Ask(cmd, &inputs.Query); err != nil {
					return err
				}
			}

			var results []docsSearchResult
			search := func() error {
				var err error
				results, err = runDocsSearch(cmd.Context(), inputs.Query, inputs.Language)
				return err
			}
			// In agent mode, skip the spinner so stderr stays clean and
			// machine-readable, matching the rest of the CLI.
			if cli.agentMode {
				if err := search(); err != nil {
					return err
				}
			} else if err := ansi.Waiting(search); err != nil {
				return err
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
				return openDocsResult(cmd, cli, results, machineFormat)
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

	// Cap the read so a malformed or oversized response cannot exhaust memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, docsSearchResponseMaxBytes))
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
	// JoinPath normalizes the separator so a leading-slash page path does not
	// produce a double slash; fall back to plain concatenation if it ever errors.
	pageURL, err := url.JoinPath(docsBaseURL, r.Page)
	if err != nil {
		pageURL = docsBaseURL + r.Page
	}
	if agentMode {
		return pageURL + ".md"
	}
	if r.Metadata.Hash != "" {
		pageURL += "#" + r.Metadata.Hash
	}
	return pageURL
}

// docsResultType returns "Doc" for prose pages, or the "<METHOD> <PATH>" parsed from the
// openapi metadata for Management API reference pages (e.g. "POST /users").
func docsResultType(r docsSearchResult) string {
	fields := strings.Fields(r.Metadata.OpenAPI)
	switch len(fields) {
	case 0:
		return "Doc"
	case 1:
		return fields[0]
	default:
		return fields[len(fields)-2] + " " + fields[len(fields)-1]
	}
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

// docsShouldPromptForResult reports whether the interactive result picker should run:
// only in an interactive terminal, with more than one result, and when no machine
// output format was requested (that output must not be clobbered by a prompt).
func docsShouldPromptForResult(interactive bool, resultCount int, machineFormat bool) bool {
	return interactive && resultCount > 1 && !machineFormat
}

// openDocsResult opens a result in the browser: interactive pick in a TTY, top result
// otherwise. It always opens the human-facing page (never the ".md" raw-markdown form),
// since opening a browser is inherently an interactive, human action. A machine output
// format (--json/--json-compact/--csv) suppresses the picker and opens the top result,
// so the emitted machine output is not clobbered by an interactive prompt.
func openDocsResult(cmd *cobra.Command, cli *cli, results []docsSearchResult, machineFormat bool) error {
	target := docsResultURL(results[0], false)
	if docsShouldPromptForResult(canPrompt(cmd), len(results), machineFormat) {
		labels := make([]string, len(results))
		byLabel := make(map[string]string, len(results))
		for i, r := range results {
			// Label as "Title (URL)"; the URL keeps labels unique when two pages
			// share a title.
			u := docsResultURL(r, false)
			label := fmt.Sprintf("%s (%s)", docsResultTitle(r), u)
			labels[i] = label
			byLabel[label] = u
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

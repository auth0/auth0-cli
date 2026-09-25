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

// docsSearchURL is the public docs search index (a var so tests can override it).
var docsSearchURL = "https://leaves.mintlify.com/api/search/auth0"

// docsBaseURL is the public docs host (a var so tests can override it).
var docsBaseURL = "https://auth0.com/"

// docsLanguage is the docs language we search; only English for now.
const docsLanguage = "en"

const docsSnippetMaxLen = 160

// docsSearchResponseMaxBytes caps the response read to avoid exhausting memory.
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

var docsSearchOpen = Flag{
	Name:     "Open",
	LongForm: "open",
	Help:     "Open a result in the browser. In an interactive terminal you pick which one; otherwise the top result opens. Not supported in agent mode.",
}

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
		Query string
		Open  bool
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
  auth0 docs search actions --open
  auth0 docs search rules --json-compact | jq '.[] | {title, url}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// An agent can't open a browser and already gets each result's ".md"
			// URL in the JSON output, so reject --open rather than guess.
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
				results, err = runDocsSearch(cmd.Context(), inputs.Query)
				return err
			}
			// Skip the spinner in agent mode to keep stderr machine-readable.
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

			// The picker only runs for the human format (and lists results itself,
			// so skip the table then); a machine format keeps its output intact.
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

	docsSearchOpen.RegisterBool(cmd, &inputs.Open, false)

	return cmd
}

// runDocsSearch queries the docs index and returns results sorted by descending
// relevance score (the API returns them unsorted, capped at ~6).
func runDocsSearch(ctx context.Context, query string) ([]docsSearchResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"query":   query,
		"filters": map[string]string{"language": docsLanguage},
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

// docsResultURL builds the public URL: agent mode gets the ".md" form (no anchor),
// human mode gets the page plus a "#hash" anchor when present.
func docsResultURL(r docsSearchResult, agentMode bool) string {
	// JoinPath avoids a double slash on leading-slash paths; fall back on error.
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

// docsResultType returns "Doc" for prose pages, or "<METHOD> <PATH>" (e.g.
// "POST /users") parsed from the openapi metadata for API reference pages.
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
	// Truncate on rune boundaries so multi-byte characters aren't split.
	if runes := []rune(content); len(runes) > docsSnippetMaxLen {
		content = strings.TrimSpace(string(runes[:docsSnippetMaxLen])) + "..."
	}
	return content
}

// docsShouldPromptForResult reports whether the picker runs: interactive terminal,
// more than one result, and no machine output format to clobber.
func docsShouldPromptForResult(interactive bool, resultCount int, machineFormat bool) bool {
	return interactive && resultCount > 1 && !machineFormat
}

// openDocsResult opens a result in the browser: interactive pick in a TTY, else the
// top result. It always opens the human page (never ".md"); a machine format
// suppresses the picker so its output isn't clobbered by a prompt.
func openDocsResult(cmd *cobra.Command, cli *cli, results []docsSearchResult, machineFormat bool) error {
	target := docsResultURL(results[0], false)
	if docsShouldPromptForResult(canPrompt(cmd), len(results), machineFormat) {
		labels := make([]string, len(results))
		byLabel := make(map[string]string, len(results))
		for i, r := range results {
			// Include the URL so labels stay unique when titles collide.
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

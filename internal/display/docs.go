package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// sectionSeparator joins breadcrumb segments in a section path.
const sectionSeparator = " > "

// shortSection collapses a long breadcrumb path for the table view, keeping the
// top-level category and the leaf page while replacing the noisy middle with an
// ellipsis (e.g. "Get Started > … > Support Readiness"). Paths of three or fewer
// segments are returned unchanged. The full path is still shown in the single-result
// detail view and in JSON output, so no information is lost for scripting.
func shortSection(section string) string {
	segments := strings.Split(section, sectionSeparator)
	if len(segments) <= 3 {
		return section
	}
	return segments[0] + sectionSeparator + "…" + sectionSeparator + segments[len(segments)-1]
}

// DocsSearchResult is the display-facing shape of a documentation search hit and
// implements View directly. The cli layer builds these (URL is already
// mode-appropriate: ".md" in agent mode).
type DocsSearchResult struct {
	Title   string
	Section string
	Type    string
	URL     string
	Snippet string
	Score   float64
}

func (v *DocsSearchResult) AsTableHeader() []string {
	return []string{"Title", "Section", "Type", "URL"}
}

func (v *DocsSearchResult) AsTableRow() []string {
	return []string{
		v.Title,
		ansi.Faint(shortSection(v.Section)),
		v.Type,
		v.URL,
	}
}

func (v *DocsSearchResult) KeyValues() [][]string {
	return [][]string{
		{"TITLE", v.Title},
		{"SECTION", v.Section},
		{"TYPE", v.Type},
		{"URL", v.URL},
		{"SNIPPET", v.Snippet},
	}
}

// docsSearchResultObject is the JSON shape emitted by --json / --json-compact. It is a
// struct (not a map) so the field order is stable and logical rather than alphabetical,
// and its json tags give lower-cased keys the exported DocsSearchResult fields lack.
type docsSearchResultObject struct {
	Title   string  `json:"title"`
	Section string  `json:"section"`
	Type    string  `json:"type"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// Object is what --json / --json-compact emit: an enriched object (not the raw API shape)
// so the mode-appropriate URL and snippet are present for scripting/agents.
func (v *DocsSearchResult) Object() interface{} {
	return docsSearchResultObject{
		Title:   v.Title,
		Section: v.Section,
		Type:    v.Type,
		URL:     v.URL,
		Snippet: v.Snippet,
		Score:   v.Score,
	}
}

// DocsSearchResults renders a list of documentation search results.
func (r *Renderer) DocsSearchResults(results []DocsSearchResult, query string) {
	resource := "documentation results"

	r.Heading(fmt.Sprintf("%s for %q (%d)", resource, query, len(results)))

	if len(results) == 0 {
		r.EmptyState(resource, "Try a different search term")
		return
	}

	res := make([]View, len(results))
	for i := range results {
		res[i] = &results[i]
	}

	// A single hit renders as a key/value detail view so the snippet is visible, but
	// only in the default human format. JSON and CSV always use the list form so their
	// output stays consistent and scriptable regardless of the number of results.
	if len(res) == 1 && r.Format == "" {
		r.Result(res[0])
		return
	}

	r.Results(res)
}

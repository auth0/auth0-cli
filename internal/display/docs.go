package display

import (
	"fmt"
)

// DocsSearchResult is the display-facing shape of a documentation search hit and
// implements View directly. The cli layer builds these (URL is already
// mode-appropriate: ".md" in agent mode).
type DocsSearchResult struct {
	Title   string
	Type    string
	URL     string
	Snippet string
	Score   float64
}

func (v *DocsSearchResult) AsTableHeader() []string {
	return []string{"Title", "Type", "URL"}
}

func (v *DocsSearchResult) AsTableRow() []string {
	return []string{
		v.Title,
		v.Type,
		v.URL,
	}
}

func (v *DocsSearchResult) KeyValues() [][]string {
	return [][]string{
		{"TITLE", v.Title},
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

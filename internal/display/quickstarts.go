package display

import (
	"fmt"
	"sort"

	"github.com/auth0/auth0-cli/internal/auth0"
)

type quickstartView struct {
	Stack   string
	AppType string
	URL     string
}

func (v *quickstartView) AsTableHeader() []string {
	return []string{"Quickstart", "Type", "URL"}
}

func (v *quickstartView) AsTableRow() []string {
	return []string{v.Stack, applyColor(v.AppType), v.URL}
}

func (r *Renderer) QuickstartList(qs map[string][]auth0.Quickstart) {
	r.Heading()

	var results []View
	keys := make([]string, 0, len(qs))

	for key := range qs {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		for _, item := range qs[key] {
			results = append(results, &quickstartView{
					Stack: item.Name,
					AppType: applyColor(qsAppTypeFor(key)),
					URL: fmt.Sprintf("https://auth0.com/docs/quickstart/%s/%s", key, item.Path),
				})
		}
	}

	r.Results(results)
}

func qsAppTypeFor(s string) string {
	switch s {
	case "native":
		return "Native"
	case "spa":
		return "Single Page Web Application"
	case "webapp":
		return "Regular Web Application"
	case "backend":
		return "Machine to Machine"
	default:
		return ""
	}
}

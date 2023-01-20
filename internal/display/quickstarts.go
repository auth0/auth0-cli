package display

import (
	"fmt"
	"sort"

	"github.com/auth0/auth0-cli/internal/auth0"
)

const qsBaseURL = "https://auth0.com"

type quickstartView struct {
	Stack   string
	AppType string
	URL     string
	raw     interface{}
}

func (v *quickstartView) AsTableHeader() []string {
	return []string{"Quickstart", "Type", "URL"}
}

func (v *quickstartView) AsTableRow() []string {
	return []string{v.Stack, applyColor(v.AppType), v.URL}
}

func (v *quickstartView) Object() interface{} {
	return v.raw
}

func (r *Renderer) QuickstartList(quickstarts []auth0.Quickstart) {
	r.Heading()

	sort.SliceStable(quickstarts, func(i, j int) bool {
		return quickstarts[i].AppType < quickstarts[j].AppType
	})

	var results []View
	for _, qs := range quickstarts {
		results = append(results, &quickstartView{
			Stack:   qs.Name,
			AppType: applyColor(qsAppTypeFor(qs.AppType)),
			URL:     fmt.Sprintf("%s%s", qsBaseURL, qs.URL),
			raw:     qs,
		})
	}

	r.Results(results)
}

func qsAppTypeFor(s string) string {
	switch s {
	case "native":
		return friendlyNative
	case "spa":
		return friendlySpa
	case "webapp":
		return friendlyReg
	case "backend":
		return friendlyM2M
	default:
		return ""
	}
}

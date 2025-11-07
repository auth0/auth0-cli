package display

import (
	"github.com/auth0/go-auth0/management"
)

type aculConfigView struct {
	ScreenName    string
	RenderingMode string
	raw           interface{}
}

func (v *aculConfigView) Object() interface{} {
	return v.raw
}

func (v *aculConfigView) AsTableHeader() []string {
	return []string{
		"Screen Name",
		"Rendering Mode",
	}
}

func (v *aculConfigView) AsTableRow() []string {
	return []string{v.ScreenName, v.RenderingMode}
}

func (r *Renderer) ACULConfigList(aculConfigs *management.PromptRenderingList) {
	resource := "prompt rendering configurations"
	r.Heading(resource)

	if len(aculConfigs.PromptRenderings) == 0 {
		r.EmptyState(resource, " Use 'auth0 acul config get' to fetch remote rendering settings or 'auth0 acul config set' to sync local configs.")
	}

	if r.Format == OutputFormatJSONCompact {
		r.JSONCompactResult(aculConfigs)
		return
	}

	if r.Format == OutputFormatJSON {
		r.JSONResult(aculConfigs)
		return
	}

	r.Results(makeACULConfigView(aculConfigs))
}

func makeACULConfigView(aculConfig *management.PromptRenderingList) []View {
	views := make([]View, 0, len(aculConfig.PromptRenderings))

	for _, v := range aculConfig.PromptRenderings {
		views = append(views, &aculConfigView{
			ScreenName:    string(*v.Screen),
			RenderingMode: string(*v.RenderingMode),
			raw:           v,
		})
	}

	return views
}

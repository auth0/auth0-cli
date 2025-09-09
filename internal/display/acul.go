package display

import "github.com/auth0/go-auth0/management"

type aculConfigView struct {
	raw interface{}
}

func (v *aculConfigView) Object() interface{} {
	return v.raw
}

func (v *aculConfigView) AsTableHeader() []string {
	return []string{}
}

func (v *aculConfigView) AsTableRow() []string {
	return []string{}
}

func (r *Renderer) ACULConfigList(aculConfigs *management.PromptRenderingList) {
	resource := "prompt rendering configurations"

	if len(aculConfigs.PromptRenderings) == 0 {
		r.EmptyState(resource, "Use 'auth0 config set' to configure acul settings for any screen")
	}

	var res []View
	for _, aculConfig := range aculConfigs.PromptRenderings {
		view := makeACULConfigView(aculConfig)

		res = append(res, view)
	}

	r.Results(res)
}

func makeACULConfigView(aculConfig *management.PromptRendering) View {
	return &aculConfigView{
		raw: aculConfig,
	}
}

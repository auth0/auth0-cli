package display

type tenantView struct {
	Name 		string
}

func (v *tenantView) AsTableHeader() []string {
	return []string{"Available tenants"}
}

func (v *tenantView) AsTableRow() []string {
	return []string{v.Name}
}

func (r *Renderer) ShowTenants(data []string) {
	var results []View
	for _, item := range data {
		results = append(results, &tenantView{
			Name: item,
		})
	}

	r.Results(results)
}

package display

type tenantView struct {
	Name string
	raw  interface{}
}

func (v *tenantView) AsTableHeader() []string {
	return []string{"Available tenants"}
}

func (v *tenantView) AsTableRow() []string {
	return []string{v.Name}
}

func (v *tenantView) Object() interface{} {
	return v.raw
}

func (r *Renderer) TenantList(data []string) {
	r.Heading()

	if len(data) == 0 {
		r.EmptyState("tenants", "Use 'auth0 login' to add one")
		return
	}

	var results []View
	for _, item := range data {
		results = append(results, &tenantView{
			Name: item,
			raw:  item,
		})
	}

	r.Results(results)
}

package display

type tenantView struct {
	Active bool
	Name   string
	raw    interface{}
}

func (v *tenantView) AsTableHeader() []string {
	return []string{"Active", "Tenant"}
}

func (v *tenantView) AsTableRow() []string {
	return []string{
		boolean(v.Active),
		v.Name,
	}
}

func (v *tenantView) Object() interface{} {
	return v.raw
}

func (r *Renderer) TenantList(data []string) {
	if len(data) == 0 {
		r.EmptyState("tenants", "Use 'auth0 login' to add one")
		return
	}

	var results []View
	for _, item := range data {
		results = append(results, &tenantView{
			Active: item == r.Tenant,
			Name:   item,
			raw:    item,
		})
	}

	r.Results(results)
}

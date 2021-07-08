package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type customDomainView struct {
	ID                   string
	Domain               string
	Status               string
	Primary              string
	ProvisioningType     string
	VerificationMethod   string
	TLSPolicy            string
	CustomClientIPHeader string
	raw                  interface{}
}

func (v *customDomainView) AsTableHeader() []string {
	return []string{"ID", "Domain", "Status"}
}

func (v *customDomainView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Domain,
		v.Status,
	}
}

func (v *customDomainView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"DOMAIN", v.Domain},
		{"STATUS", v.Status},
		{"PRIMARY", v.Primary},
		{"PROVISIONING TYPE", v.ProvisioningType},
		{"VERIFICATION METHOD", v.VerificationMethod},
		{"TLS POLICY", v.TLSPolicy},
		{"CUSTOM CLIENT IP HEADER", v.CustomClientIPHeader},
	}
}

func (v *customDomainView) Object() interface{} {
	return v.raw
}

func (r *Renderer) CustomDomainList(customDomains []*management.CustomDomain) {
	resource := "custom domains"

	r.Heading(resource)

	if len(customDomains) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 branding domains create' to add one")
		return
	}

	var res []View
	for _, customDomain := range customDomains {
		res = append(res, makeCustomDomainView(customDomain))
	}

	r.Results(res)
}

func (r *Renderer) CustomDomainShow(customDomain *management.CustomDomain) {
	r.Heading("custom domain")
	r.Result(makeCustomDomainView(customDomain))
}

func (r *Renderer) CustomDomainCreate(customDomain *management.CustomDomain) {
	r.Heading("custom domain created")
	r.Result(makeCustomDomainView(customDomain))
}

func (r *Renderer) CustomDomainUpdate(customDomain *management.CustomDomain) {
	r.Heading("custom domain updated")
	r.Result(makeCustomDomainView(customDomain))
}

func makeCustomDomainView(customDomain *management.CustomDomain) *customDomainView {
	return &customDomainView{
		ID:                   ansi.Faint(customDomain.GetID()),
		Domain:               customDomain.GetDomain(),
		Status:               customDomainStatusColor(customDomain.GetStatus()),
		Primary:              boolean(customDomain.GetPrimary()),
		ProvisioningType:     customDomain.GetType(),
		VerificationMethod:   customDomain.GetVerificationMethod(),
		TLSPolicy:            customDomain.GetTLSPolicy(),
		CustomClientIPHeader: customDomain.GetCustomClientIPHeader(),
		raw:                  customDomain,
	}
}

func customDomainStatusColor(v string) string {
	switch v {
	case "disabled":
		return ansi.Red(v)
	case "pending", "pending_verification":
		return ansi.Yellow(v)
	case "ready":
		return ansi.Green(v)
	default:
		return v
	}
}

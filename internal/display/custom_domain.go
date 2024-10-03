package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type customDomainView struct {
	ID                   string
	Domain               string
	Status               string
	Primary              string
	ProvisioningType     string
	VerificationMethod   string
	VerificationRecord   string
	VerificationDomain   string
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
	var keyValues [][]string

	if v.ID != "" {
		keyValues = append(keyValues, []string{"ID", ansi.Faint(v.ID)})
	}
	if v.Domain != "" {
		keyValues = append(keyValues, []string{"DOMAIN", v.Domain})
	}
	if v.Status != "" {
		keyValues = append(keyValues, []string{"STATUS", v.Status})
	}
	if v.Primary != "" {
		keyValues = append(keyValues, []string{"PRIMARY", v.Primary})
	}
	if v.ProvisioningType != "" {
		keyValues = append(keyValues, []string{"PROVISIONING TYPE", v.ProvisioningType})
	}
	if v.VerificationMethod != "" {
		keyValues = append(keyValues, []string{ansi.Cyan(ansi.Bold("VERIFICATION METHOD")), ansi.Cyan(ansi.Bold(v.VerificationMethod))})
	}
	if v.VerificationRecord != "" {
		keyValues = append(keyValues, []string{ansi.Cyan(ansi.Bold("VERIFICATION RECORD VALUE")), ansi.Cyan(ansi.Bold(v.VerificationRecord))})
	}
	if v.VerificationDomain != "" {
		keyValues = append(keyValues, []string{ansi.Cyan(ansi.Bold("VERIFICATION DOMAIN")), ansi.Cyan(ansi.Bold(v.VerificationDomain))})
	}
	if v.TLSPolicy != "" {
		keyValues = append(keyValues, []string{"TLS POLICY", v.TLSPolicy})
	}
	if v.CustomClientIPHeader != "" {
		keyValues = append(keyValues, []string{"CUSTOM CLIENT IP HEADER", v.CustomClientIPHeader})
	}

	return keyValues
}

func (v *customDomainView) Object() interface{} {
	return v.raw
}

func (r *Renderer) CustomDomainList(customDomains []*management.CustomDomain) {
	resource := "custom domains"

	r.Heading(resource)

	if len(customDomains) == 0 {
		r.EmptyState(resource, "Use 'auth0 domains create' to add one")
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
	view := &customDomainView{
		ID:                   ansi.Faint(customDomain.GetID()),
		Domain:               customDomain.GetDomain(),
		Status:               customDomainStatusColor(customDomain.GetStatus()),
		Primary:              boolean(customDomain.GetPrimary()),
		ProvisioningType:     customDomain.GetType(),
		TLSPolicy:            customDomain.GetTLSPolicy(),
		CustomClientIPHeader: customDomain.GetCustomClientIPHeader(),
		raw:                  customDomain,
	}

	if len(customDomain.GetVerification().Methods) > 0 {
		method := customDomain.GetVerification().Methods[0]
		if name, ok := method["name"].(string); ok {
			view.VerificationMethod = name
		}
		if record, ok := method["record"].(string); ok {
			view.VerificationRecord = record
		}
		if domain, ok := method["domain"].(string); ok {
			view.VerificationDomain = domain
		}
	}

	return view
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

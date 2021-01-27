package display

import (
	"strconv"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type customDomainView struct {
	ID                 string
	Domain             string
	Type               string
	Primary            bool
	Status             string
	VerificationMethod string
	Verification       customDomainVerificationView
}

type customDomainVerificationView struct {
	Methods []map[string]interface{}
}

func (v *customDomainView) AsTableHeader() []string {
	return []string{"Domain", "Custom Domain ID", "Type", "Primary", "Status", "Verification Method"}
}

func (v *customDomainView) AsTableRow() []string {
	return []string{v.Domain, v.ID, v.Type, strconv.FormatBool(v.Primary), v.Status, v.VerificationMethod}
}

func (r *Renderer) CustomDomainList(customDomains []*management.CustomDomain) {
	r.Heading(ansi.Bold(r.Tenant), "custom-domains\n")
	var res []View
	for _, c := range customDomains {
		res = append(res, &customDomainView{
			ID:                 auth0.StringValue(c.ID),
			Domain:             auth0.StringValue(c.Domain),
			Type:               auth0.StringValue(c.Type),
			Primary:            auth0.BoolValue(c.Primary),
			Status:             auth0.StringValue(c.Status),
			VerificationMethod: auth0.StringValue(c.VerificationMethod),
		})
	}
	r.Results(res)
}

func (r *Renderer) CustomDomainCreate(customDomain *management.CustomDomain) {
	r.Heading(ansi.Bold(r.Tenant), "custom-domain created\n")
	r.Results([]View{&customDomainView{
		Domain:             auth0.StringValue(customDomain.Domain),
		Type:               auth0.StringValue(customDomain.Type),
		ID:                 auth0.StringValue(customDomain.ID),
		Primary:            auth0.BoolValue(customDomain.Primary),
		Status:             auth0.StringValue(customDomain.Status),
		VerificationMethod: auth0.StringValue(customDomain.VerificationMethod),
	}})
}

func (r *Renderer) CustomDomainGet(customDomain *management.CustomDomain) {
	r.Heading(ansi.Bold(r.Tenant), "custom-domain\n")
	r.Results([]View{&customDomainView{
		Domain:             auth0.StringValue(customDomain.Domain),
		Type:               auth0.StringValue(customDomain.Type),
		ID:                 auth0.StringValue(customDomain.ID),
		Primary:            auth0.BoolValue(customDomain.Primary),
		Status:             auth0.StringValue(customDomain.Status),
		VerificationMethod: auth0.StringValue(customDomain.VerificationMethod),
	}})
}

func (r *Renderer) CustomDomainVerify(customDomain *management.CustomDomain) {
	r.Heading(ansi.Bold(r.Tenant), "custom-domain verified\n")
	r.Results([]View{&customDomainView{
		Domain:             auth0.StringValue(customDomain.Domain),
		Type:               auth0.StringValue(customDomain.Type),
		ID:                 auth0.StringValue(customDomain.ID),
		Primary:            auth0.BoolValue(customDomain.Primary),
		Status:             auth0.StringValue(customDomain.Status),
		VerificationMethod: auth0.StringValue(customDomain.VerificationMethod),
	}})
}

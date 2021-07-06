package display

import (
	"encoding/json"
	"io"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type organizationView struct {
	ID              string
	Name            string
	DisplayName     string
	LogoURL         string
	AccentColor     string
	BackgroundColor string
	Metadata        string
	raw             interface{}
}

func (v *organizationView) AsTableHeader() []string {
	return []string{"ID", "Name", "Display Name"}
}

func (v *organizationView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.DisplayName}
}

func (v *organizationView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"DISPLAY NAME", v.DisplayName},
		{"LOGO URL", v.LogoURL},
		{"ACCENT COLOR", v.AccentColor},
		{"BACKGROUND COLOR", v.BackgroundColor},
		{"METADATA", v.Metadata},
	}
}

func (v *organizationView) Object() interface{} {
	return v.raw
}

func (r *Renderer) OrganizationList(organizations []*management.Organization) {
	resource := "organizations"

	r.Heading(resource)

	if len(organizations) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 orgs create' to add one")
		return
	}

	var res []View
	for _, o := range organizations {
		res = append(res, makeOrganizationView(o, r.MessageWriter))
	}

	r.Results(res)
}

func (r *Renderer) OrganizationShow(organization *management.Organization) {
	r.Heading("organization")
	r.Result(makeOrganizationView(organization, r.MessageWriter))
}

func (r *Renderer) OrganizationCreate(organization *management.Organization) {
	r.Heading("organization created")
	r.Result(makeOrganizationView(organization, r.MessageWriter))
}

func (r *Renderer) OrganizationUpdate(organization *management.Organization) {
	r.Heading("organization updated")
	r.Result(makeOrganizationView(organization, r.MessageWriter))
}

func makeOrganizationView(organization *management.Organization, w io.Writer) *organizationView {
	accentColor := ""
	backgroundColor := ""

	if organization.Branding != nil && organization.Branding.Colors != nil {
		if len(organization.Branding.Colors["primary"]) > 0 {
			accentColor = organization.Branding.Colors["primary"]
		}

		if len(organization.Branding.Colors["page_background"]) > 0 {
			backgroundColor = organization.Branding.Colors["page_background"]
		}
	}

	metadata := ""
	buf, err := json.MarshalIndent(organization.Metadata, "", "    ")
	if err == nil {
		metadata = string(buf)
	}

	return &organizationView{
		ID:              organization.GetID(),
		Name:            organization.GetName(),
		DisplayName:     organization.GetDisplayName(),
		LogoURL:         organization.GetBranding().GetLogoUrl(),
		AccentColor:     accentColor,
		BackgroundColor: backgroundColor,
		Metadata:        ansi.ColorizeJSON(metadata, false, w),
		raw:             organization,
	}
}

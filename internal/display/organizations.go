package display

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type organizationView struct {
	ID          string
	Name        string
	DisplayName string
	LogoURL     string
	Colors      string
	Metadata    string
	raw         interface{}
}

func (v *organizationView) AsTableHeader() []string {
	return []string{"ID", "Name", "DisplayName"}
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
		{"COLORS", v.Colors},
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
		res = append(res, &organizationView{
			ID:          o.GetID(),
			Name:        o.GetName(),
			DisplayName: o.GetDisplayName(),
			raw:         o,
		})
	}

	r.Results(res)
}

func (r *Renderer) OrganizationShow(organization *management.Organization) {
	r.Heading("organization")
	r.Result(makeOrganizationView(organization))
}

func (r *Renderer) OrganizationCreate(organization *management.Organization) {
	r.Heading("organization created")
	r.Result(makeOrganizationView(organization))
}

func (r *Renderer) OrganizationUpdate(organization *management.Organization) {
	r.Heading("organization updated")
	r.Result(makeOrganizationView(organization))
}

func makeOrganizationView(organization *management.Organization) *organizationView {
	var colors = make([]string, 0, len(organization.GetBranding().Colors))
	for k, v := range organization.GetBranding().Colors {
		colors = append(colors, fmt.Sprintf("%s: %s", k, v))
	}

	metadata := ""
	buf, err := json.MarshalIndent(organization.Metadata, "", "    ")
	if err != nil {
		metadata = string(buf)
	}

	return &organizationView{
		ID:          organization.GetID(),
		Name:        organization.GetName(),
		DisplayName: organization.GetDisplayName(),
		LogoURL:     organization.GetBranding().GetLogoUrl(),
		Colors:      strings.Join(colors, "\n"),
		Metadata:    metadata,
		raw:         organization,
	}
}


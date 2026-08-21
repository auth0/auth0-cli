package display

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// clientGrantOrganizationView renders a single organization associated with a
// client grant as a row in the list table.
type clientGrantOrganizationView struct {
	ID          string
	Name        string
	DisplayName string

	raw interface{}
}

func (v *clientGrantOrganizationView) AsTableHeader() []string {
	return []string{"ID", "Name", "Display Name"}
}

func (v *clientGrantOrganizationView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.DisplayName}
}

func (v *clientGrantOrganizationView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ClientGrantOrganizationList(organizations []*managementv3.Organization) {
	resource := "client grant organizations"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(organizations)))

	if len(organizations) == 0 {
		r.EmptyState(resource, "This client grant is not associated with any organizations")
		return
	}

	var results []View
	for _, organization := range organizations {
		results = append(results, &clientGrantOrganizationView{
			ID:          organization.GetID(),
			Name:        organization.GetName(),
			DisplayName: organization.GetDisplayName(),
			raw:         organization,
		})
	}

	r.Results(results)
}

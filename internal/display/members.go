package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type membersView struct {
	ID         string
	Name       string
	Email      string
	PictureURL string
	raw        interface{}
}

func (v *membersView) AsTableHeader() []string {
	return []string{"ID", "Name", "Email", "Picture"}
}

func (v *membersView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Email, v.PictureURL}
}

func (v *membersView) KeyValues() [][]string {
	return [][]string{}
}

func (v *membersView) Object() interface{} {
	return v.raw
}

func (r *Renderer) MembersList(members []management.OrganizationMember) {
	resource := "organization members"

	r.Heading(resource)

	if len(members) == 0 {
		r.EmptyState(resource, "")
		return
	}

	var res []View
	for _, m := range members {
		res = append(res, makeMembersView(m))
	}

	r.Results(res)
}

func makeMembersView(member management.OrganizationMember) *membersView {
	return &membersView{
		ID:         member.GetUserID(),
		Name:       member.GetName(),
		Email:      member.GetEmail(),
		PictureURL: member.GetPicture(),
		raw:        member,
	}
}

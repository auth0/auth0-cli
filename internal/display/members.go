package display

import (
	"io"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
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
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"Email", v.Email},
		{"Picture URL", v.PictureURL},
	}
}

func (v *membersView) Object() interface{} {
	return v.raw
}

func (r *Renderer) MembersList(members []management.OrganizationMember) {
	resource := "members"

	r.Heading(resource)

	var res []View
	for _, m := range members {
		res = append(res, makeMembersView(&m, r.MessageWriter))
	}

	r.Results(res)
}

func makeMembersView(member *management.OrganizationMember, w io.Writer) *membersView {

	return &membersView{
		ID:              member.GetUserID(),
		Name:            member.GetName(),
		Email:           member.GetEmail(),
		PictureURL:      member.GetPicture(),
		raw:             member,
	}
}

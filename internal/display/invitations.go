package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type invitationsView struct {
	ID           string
	ClientID     string
	ConnectionID string
	InviterName  string
	InviteeEmail string
	ExpiresAt    string
	raw          interface{}
}

func (v *invitationsView) AsTableHeader() []string {
	return []string{"ID", "Client ID", "Connection ID", "Inviter Name", "Invitee Email", "Expires At"}
}

func (v *invitationsView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.ClientID, v.ConnectionID, v.InviterName, v.InviteeEmail, v.ExpiresAt}
}

func (v *invitationsView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"CLIENT ID", v.ClientID},
		{"CONNECTION ID", v.ConnectionID},
		{"INVITER NAME", v.InviterName},
		{"INVITEE EMAIL", v.InviteeEmail},
		{"EXPIRES AT", v.ExpiresAt},
	}
}

func (v *invitationsView) Object() interface{} {
	return v.raw
}

func (r *Renderer) InvitationsList(invitations []management.OrganizationInvitation) {
	resource := "organization invitations"

	r.Heading(resource)

	if len(invitations) == 0 {
		r.EmptyState(resource, "Use 'auth0 orgs invs create' to add one")
		return
	}

	var res []View
	for _, m := range invitations {
		res = append(res, makeInvitationsView(m))
	}

	r.Results(res)
}

func (r *Renderer) InvitationsCreate(invitation management.OrganizationInvitation) {
	r.Heading("organization invitation created")
	r.Result(makeInvitationsView(invitation))
}

func (r *Renderer) InvitationsShow(invitation management.OrganizationInvitation) {
	r.Heading("organization invitation")
	r.Result(makeInvitationsView(invitation))
}

func makeInvitationsView(invitation management.OrganizationInvitation) *invitationsView {
	return &invitationsView{
		ID:           invitation.GetID(),
		InviterName:  invitation.GetInviter().GetName(),
		InviteeEmail: invitation.GetInvitee().GetEmail(),
		ExpiresAt:    invitation.GetExpiresAt(),
		ClientID:     invitation.GetClientID(),
		ConnectionID: invitation.GetConnectionID(),
		raw:          invitation,
	}
}

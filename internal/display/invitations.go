package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type invitationsView struct {
	ID           string
	ClientId     string
	ConnectionId string
	InviterName  string
	InviteeEmail string
	CreatedAt    string
	ExpiresAt    string
	raw          interface{}
	// TODO: Check if OrganizationId, InvitationURL, Role assignments are needed
}

func (v *invitationsView) AsTableHeader() []string {
	return []string{"ID", "Client ID", "Connection ID", "Inviter Name", "Invitee Email", "Created At", "Expires At"}
}

func (v *invitationsView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.ClientId, v.ConnectionId, v.InviterName, v.InviteeEmail, v.CreatedAt, v.ExpiresAt}
}

func (v *invitationsView) KeyValues() [][]string {
	return [][]string{}
}

func (v *invitationsView) Object() interface{} {
	return v.raw
}

func (r *Renderer) InvitationsList(invitations []management.OrganizationInvitation) {
	resource := "organization invitations"

	r.Heading(resource)

	if len(invitations) == 0 {
		r.EmptyState(resource, "")
		return
	}

	var res []View
	for _, m := range invitations {
		res = append(res, makeInvitationsView(m))
	}

	r.Results(res)
}

func makeInvitationsView(invitation management.OrganizationInvitation) *invitationsView {
	return &invitationsView{
		ID:           invitation.GetID(),
		InviterName:  invitation.GetInviter().GetName(),
		InviteeEmail: invitation.GetInvitee().GetEmail(),
		CreatedAt:    invitation.GetCreatedAt(),
		ExpiresAt:    invitation.GetExpiresAt(),
		ClientId:     invitation.GetClientID(),
		ConnectionId: invitation.GetConnectionID(),
		raw:          invitation,
	}
}

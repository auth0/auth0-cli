package display

import (
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func Test_invitationsView_AsTableHeader(t *testing.T) {
	mockInvitationsView := invitationsView{}

	assert.Equal(t, []string{"ID", "Client ID", "Connection ID", "Inviter Name", "Invitee Email", "Expires At"}, mockInvitationsView.AsTableHeader())
}

func Test_invitationsView_AsTableRow(t *testing.T) {
	mockInvitationsView := invitationsView{
		ID:           "invitation-id",
		ClientID:     "client-id",
		ConnectionID: "connection-id",
		InviterName:  "inviter-name",
		InviteeEmail: "invitee-email",
		ExpiresAt:    "expires-at",
	}

	assert.Equal(t, []string{"invitation-id", "client-id", "connection-id", "inviter-name", "invitee-email", "expires-at"}, mockInvitationsView.AsTableRow())
}

func Test_invitationsView_KeyValues(t *testing.T) {
	mockInvitationsView := invitationsView{
		ID:           "invitation-id",
		ClientID:     "client-id",
		ConnectionID: "connection-id",
		InviterName:  "inviter-name",
		InviteeEmail: "invitee-email",
		ExpiresAt:    "expires-at",
	}

	expected := [][]string{
		{"ID", "invitation-id"},
		{"CLIENT ID", "client-id"},
		{"CONNECTION ID", "connection-id"},
		{"INVITER NAME", "inviter-name"},
		{"INVITEE EMAIL", "invitee-email"},
		{"EXPIRES AT", "expires-at"},
	}

	assert.Equal(t, expected, mockInvitationsView.KeyValues())
}

func Test_invitationsView_Object(t *testing.T) {
	mockInvitationsView := invitationsView{
		raw: "raw data",
	}

	assert.Equal(t, "raw data", mockInvitationsView.Object())
}

func Test_makeInvitationsView(t *testing.T) {
	mockInvitation := management.OrganizationInvitation{
		ID: auth0.String("invitation-id"),
		Inviter: &management.OrganizationInvitationInviter{
			Name: auth0.String("inviter-name"),
		},
		Invitee: &management.OrganizationInvitationInvitee{
			Email: auth0.String("invitee-email"),
		},
		ExpiresAt:    auth0.String("expires-at"),
		ClientID:     auth0.String("client-id"),
		ConnectionID: auth0.String("connection-id"),
	}

	view := makeInvitationsView(mockInvitation)

	assert.Equal(t, "invitation-id", view.ID)
	assert.Equal(t, "client-id", view.ClientID)
	assert.Equal(t, "connection-id", view.ConnectionID)
	assert.Equal(t, "inviter-name", view.InviterName)
	assert.Equal(t, "invitee-email", view.InviteeEmail)
	assert.Equal(t, "expires-at", view.ExpiresAt)
	assert.Equal(t, mockInvitation, view.raw)
}

package display

import (
	"testing"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func TestMakeSessionTransferView_WithoutDelegation(t *testing.T) {
	client := &management.Client{
		ClientID: auth0.String("client-id"),
		SessionTransfer: &management.SessionTransfer{
			CanCreateSessionTransferToken: auth0.Bool(true),
			AllowedAuthenticationMethods:  &[]string{"cookie", "query"},
			EnforceDeviceBinding:          auth0.String("ip"),
		},
	}

	view := MakeSessionTransferView(client)

	assert.False(t, view.hasDelegation)
	assert.Equal(t, "", view.DelegationAllowAccess)
	assert.Equal(t, "", view.DelegationDeviceBinding)

	// Delegation rows must be omitted when no delegation is configured.
	keyValues := view.KeyValues()
	assert.Equal(t, [][]string{
		{"CLIENT ID", "client-id"},
		{"CAN CREATE TOKEN", boolean(true)},
		{"ALLOWED METHODS", "cookie, query"},
		{"DEVICE BINDING", "ip"},
	}, keyValues)
}

func TestMakeSessionTransferView_WithDelegation(t *testing.T) {
	client := &management.Client{
		ClientID: auth0.String("client-id"),
		SessionTransfer: &management.SessionTransfer{
			CanCreateSessionTransferToken: auth0.Bool(true),
			AllowedAuthenticationMethods:  &[]string{"cookie"},
			EnforceDeviceBinding:          auth0.String("ip"),
			Delegation: &management.SessionTransferDelegation{
				AllowDelegatedAccess: auth0.Bool(true),
				EnforceDeviceBinding: auth0.String("asn"),
			},
		},
	}

	view := MakeSessionTransferView(client)

	assert.True(t, view.hasDelegation)
	assert.Equal(t, boolean(true), view.DelegationAllowAccess)
	assert.Equal(t, "asn", view.DelegationDeviceBinding)

	// Delegation rows must be appended after the base session-transfer rows.
	keyValues := view.KeyValues()
	assert.Equal(t, [][]string{
		{"CLIENT ID", "client-id"},
		{"CAN CREATE TOKEN", boolean(true)},
		{"ALLOWED METHODS", "cookie"},
		{"DEVICE BINDING", "ip"},
		{"ALLOW DELEGATED ACCESS", boolean(true)},
		{"DELEGATION DEVICE BINDING", "asn"},
	}, keyValues)
}

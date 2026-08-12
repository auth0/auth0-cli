package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type SessionTransferView struct {
	ID             string
	CanCreateTOKEN string
	AllowedMethods string
	DeviceBinding  string

	// Delegation (EA) fields, shown only when hasDelegation is true.
	hasDelegation           bool
	DelegationAllowAccess   string
	DelegationDeviceBinding string

	raw interface{}
}

func (v *SessionTransferView) AsTableHeader() []string {
	return []string{"CLIENT ID", "NAME", "CAN CREATE TOKEN", "ALLOWED METHODS", "DEVICE BINDING"}
}

func (v *SessionTransferView) AsTableRow() []string {
	return []string{
		v.ID,
		v.CanCreateTOKEN,
		v.AllowedMethods,
		v.DeviceBinding,
	}
}

func (v *SessionTransferView) KeyValues() [][]string {
	keyValues := [][]string{
		{"CLIENT ID", v.ID},
		{"CAN CREATE TOKEN", v.CanCreateTOKEN},
		{"ALLOWED METHODS", v.AllowedMethods},
		{"DEVICE BINDING", v.DeviceBinding},
	}

	if v.hasDelegation {
		keyValues = append(keyValues,
			[]string{"ALLOW DELEGATED ACCESS", v.DelegationAllowAccess},
			[]string{"DELEGATION DEVICE BINDING", v.DelegationDeviceBinding},
		)
	}

	return keyValues
}

func (v *SessionTransferView) Object() interface{} {
	return v.raw
}

func (r *Renderer) SessionTransferShow(client *management.Client) {
	r.Heading("application session transfer")
	r.Result(MakeSessionTransferView(client))
}

func (r *Renderer) SessionTransferUpdate(client *management.Client, id string) {
	r.Heading("application session transfer")
	r.Infof("✅ Updated session transfer settings for application %s", ansi.Faint(id))

	r.Result(MakeSessionTransferView(client))
}

func MakeSessionTransferView(client *management.Client) *SessionTransferView {
	view := &SessionTransferView{
		ID:             client.GetClientID(),
		CanCreateTOKEN: boolean(client.SessionTransfer.GetCanCreateSessionTransferToken()),
		AllowedMethods: stringSliceToCommaSeparatedString(client.SessionTransfer.GetAllowedAuthenticationMethods()),
		DeviceBinding:  client.SessionTransfer.GetEnforceDeviceBinding(),
		raw:            client.SessionTransfer,
	}

	if delegation := client.GetSessionTransfer().GetDelegation(); delegation != nil {
		view.hasDelegation = true
		view.DelegationAllowAccess = boolean(delegation.GetAllowDelegatedAccess())
		view.DelegationDeviceBinding = delegation.GetEnforceDeviceBinding()
	}

	return view
}

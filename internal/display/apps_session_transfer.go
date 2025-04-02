package display

import (
	"fmt"
	"strings"
"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/go-auth0/management"
)

type SessionTransferView struct {
	Client *management.Client
	raw interface{}
}

func (v *SessionTransferView) AsTableHeader() []string {
	return []string{"CLIENT ID", "NAME", "CAN CREATE TOKEN", "ALLOWED METHODS", "DEVICE BINDING"}
}

func (v *SessionTransferView) AsTableRow() []string {
	st := v.Client.SessionTransfer
	return []string{
		ansi.Faint(v.Client.GetClientID()),
		v.Client.GetName(),
		fmt.Sprintf("%v", derefBool(st.CanCreateSessionTransferToken)),
		strings.Join(derefStringSlice(st.AllowedAuthenticationMethods), ", "),
		derefString(st.EnforceDeviceBinding),
	}
}



func (v *SessionTransferView) KeyValues() [][]string {
	st := v.Client.SessionTransfer
	return [][]string{
		{"CLIENT ID", ansi.Faint(v.Client.GetClientID())},
		{"NAME", v.Client.GetName()},
		{"CAN CREATE TOKEN", fmt.Sprintf("%v", derefBool(st.CanCreateSessionTransferToken))},
		{"ALLOWED METHODS", strings.Join(derefStringSlice(st.AllowedAuthenticationMethods), ", ")},
		{"DEVICE BINDING", derefString(st.EnforceDeviceBinding)},
	}
}

func (v *SessionTransferView) Object() interface{} {
	return v.raw
}

func (r *Renderer) SessionTransferShow(client *management.Client) {
	r.Heading("application session transfer")
	r.Result(MakeSessionTransferView(client))
}

func MakeSessionTransferView(client *management.Client) *SessionTransferView {
	return &SessionTransferView{
		Client: client,
		raw:    client.SessionTransfer,
	}
}

// Helpers used here instead of auth0 package utils
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func derefStringSlice(s *[]string) []string {
	if s == nil {
		return nil
	}
	return *s
}

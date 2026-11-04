package display

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// refreshTokenResponse is satisfied by every refresh token response content
// type returned by the v3 SDK (list, get and update), which all share the same
// getter surface. It lets a single view constructor serve all of them.
type refreshTokenResponse interface {
	GetID() string
	GetUserID() string
	GetClientID() string
	GetSessionID() managementv3.RefreshTokenSessionID
	GetRotating() bool
	GetCreatedAt() managementv3.RefreshTokenDate
	GetExpiresAt() managementv3.RefreshTokenDate
	GetLastExchangedAt() managementv3.RefreshTokenDate
	GetDevice() managementv3.RefreshTokenDevice
}

type refreshTokenView struct {
	ID           string
	UserID       string
	ClientID     string
	SessionID    string
	Rotating     string
	Device       string
	CreatedAt    string
	LastExchange string
	ExpiresAt    string

	raw interface{}
}

func (v *refreshTokenView) AsTableHeader() []string {
	return []string{"ID", "User ID", "Client ID", "Session ID", "Rotating", "Expires At"}
}

func (v *refreshTokenView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.UserID,
		v.ClientID,
		v.SessionID,
		v.Rotating,
		v.ExpiresAt,
	}
}

func (v *refreshTokenView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"USER ID", v.UserID},
		{"CLIENT ID", v.ClientID},
		{"SESSION ID", v.SessionID},
		{"ROTATING", v.Rotating},
		{"DEVICE", v.Device},
		{"CREATED AT", v.CreatedAt},
		{"LAST EXCHANGED", v.LastExchange},
		{"EXPIRES AT", v.ExpiresAt},
	}
}

func (v *refreshTokenView) Object() interface{} {
	return v.raw
}

func (r *Renderer) RefreshTokenList(tokens []*managementv3.RefreshTokenResponseContent) {
	resource := "refresh tokens"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(tokens)))

	if len(tokens) == 0 {
		r.EmptyState(resource, "This user has no refresh tokens")
		return
	}

	var results []View
	for _, token := range tokens {
		results = append(results, makeRefreshTokenView(token))
	}

	r.Results(results)
}

func (r *Renderer) RefreshTokenShow(token *managementv3.GetRefreshTokenResponseContent) {
	r.Heading("refresh token")
	r.Result(makeRefreshTokenView(token))
}

func (r *Renderer) RefreshTokenUpdate(token *managementv3.UpdateRefreshTokenResponseContent) {
	r.Heading("refresh token updated")
	r.Result(makeRefreshTokenView(token))
}

func makeRefreshTokenView(token refreshTokenResponse) *refreshTokenView {
	device := token.GetDevice()

	sessionID := "-"
	if id := token.GetSessionID(); id != nil {
		sessionID = *id
	}

	return &refreshTokenView{
		ID:           token.GetID(),
		UserID:       token.GetUserID(),
		ClientID:     token.GetClientID(),
		SessionID:    sessionID,
		Rotating:     boolean(token.GetRotating()),
		Device:       device.GetLastUserAgent(),
		CreatedAt:    refreshTokenDateForDisplay(token.GetCreatedAt()),
		LastExchange: refreshTokenDateForDisplay(token.GetLastExchangedAt()),
		ExpiresAt:    refreshTokenDateForDisplay(token.GetExpiresAt()),
		raw:          token,
	}
}

// refreshTokenDateForDisplay renders a RefreshTokenDate union for display,
// deferring to dateForDisplay so unset, past and future dates are all handled
// consistently with the session views.
func refreshTokenDateForDisplay(date managementv3.RefreshTokenDate) string {
	return dateForDisplay(date.GetDateTime())
}

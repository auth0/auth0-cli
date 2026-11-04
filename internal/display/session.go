package display

import (
	"fmt"
	"time"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// sessionResponse is satisfied by every session response content type returned
// by the v3 SDK (list, get and update), which all share the same getter
// surface. It lets a single view constructor serve all of them.
type sessionResponse interface {
	GetID() string
	GetUserID() string
	GetExpiresAt() managementv3.SessionDate
	GetLastInteractedAt() managementv3.SessionDate
	GetCreatedAt() managementv3.SessionDate
	GetDevice() managementv3.SessionDeviceMetadata
}

type sessionView struct {
	ID             string
	UserID         string
	Device         string
	LastInteracted string
	ExpiresAt      string
	CreatedAt      string

	raw interface{}
}

func (v *sessionView) AsTableHeader() []string {
	return []string{"ID", "User ID", "Device", "Last Interacted", "Expires At"}
}

func (v *sessionView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.UserID,
		v.Device,
		v.LastInteracted,
		v.ExpiresAt,
	}
}

func (v *sessionView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"USER ID", v.UserID},
		{"DEVICE", v.Device},
		{"LAST INTERACTED", v.LastInteracted},
		{"CREATED AT", v.CreatedAt},
		{"EXPIRES AT", v.ExpiresAt},
	}
}

func (v *sessionView) Object() interface{} {
	return v.raw
}

func (r *Renderer) SessionList(sessions []*managementv3.SessionResponseContent) {
	resource := "sessions"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(sessions)))

	if len(sessions) == 0 {
		r.EmptyState(resource, "This user has no active sessions")
		return
	}

	var results []View
	for _, session := range sessions {
		results = append(results, makeSessionView(session))
	}

	r.Results(results)
}

func (r *Renderer) SessionShow(session *managementv3.GetSessionResponseContent) {
	r.Heading("session")
	r.Result(makeSessionView(session))
}

func (r *Renderer) SessionUpdate(session *managementv3.UpdateSessionResponseContent) {
	r.Heading("session updated")
	r.Result(makeSessionView(session))
}

func makeSessionView(session sessionResponse) *sessionView {
	device := session.GetDevice()

	return &sessionView{
		ID:             session.GetID(),
		UserID:         session.GetUserID(),
		Device:         device.GetLastUserAgent(),
		LastInteracted: sessionDateForDisplay(session.GetLastInteractedAt()),
		CreatedAt:      sessionDateForDisplay(session.GetCreatedAt()),
		ExpiresAt:      sessionDateForDisplay(session.GetExpiresAt()),
		raw:            session,
	}
}

// sessionDateForDisplay unwraps a SessionDate union and defers to dateForDisplay
// so unset, past and future dates are all handled consistently with the refresh
// token views.
func sessionDateForDisplay(date managementv3.SessionDate) string {
	return dateForDisplay(date.GetDateTime())
}

// dateForDisplay renders a timestamp for session and refresh token views. An
// unset (zero) date reads as a dash; a past date reads as a relative time; a
// future date (such as an expiry) reads as an absolute date, because timeAgo
// only makes sense for timestamps in the past.
func dateForDisplay(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	if t.After(time.Now()) {
		return t.Format("Jan 02 2006")
	}
	return timeAgo(t)
}

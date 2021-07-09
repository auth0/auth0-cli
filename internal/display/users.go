package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type userView struct {
	UserID          string
	Email           string
	Connection      string
	Username        string
	RequireUsername bool
	raw             interface{}
}

func (v *userView) AsTableHeader() []string {
	return []string{
		"UserID",
		"Email",
		"Connection",
	}
}

func (v *userView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.UserID),
		v.Email,
		v.Connection,
	}
}

func (v *userView) KeyValues() [][]string {
	if v.RequireUsername {
		return [][]string{
			{"ID", ansi.Faint(v.UserID)},
			{"EMAIL", v.Email},
			{"CONNECTION", v.Connection},
			{"USERNAME", v.Username},
		}
	}
	return [][]string{
		{"ID", ansi.Faint(v.UserID)},
		{"EMAIL", v.Email},
		{"CONNECTION", v.Connection},
	}
}

func (v *userView) Object() interface{} {
	return v.raw
}

func (r *Renderer) UserSearch(users []*management.User) {
	resource := "user"

	r.Heading(resource)

	if len(users) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 users create' to add one")
		return
	}

	var res []View
	for _, user := range users {
		res = append(res, makeUserView(user, false))
	}

	r.Results(res)
}

func (r *Renderer) UserShow(user *management.User, requireUsername bool) {
	r.Heading("user")
	r.Result(makeUserView(user, requireUsername))
}

func (r *Renderer) UserCreate(user *management.User, requireUsername bool) {
	r.Heading("user created")
	r.Result(makeUserView(user, requireUsername))
}

func (r *Renderer) UserUpdate(user *management.User, requireUsername bool) {
	r.Heading("user updated")
	r.Result(makeUserView(user, requireUsername))
}

func makeUserView(user *management.User, requireUsername bool) *userView {
	return &userView{
		RequireUsername: requireUsername,
		UserID:          ansi.Faint(auth0.StringValue(user.ID)),
		Email:           auth0.StringValue(user.Email),
		Connection:      stringSliceToCommaSeparatedString(getUserConnection(user)),
		Username:        auth0.StringValue(user.Username),
		raw:             user,
	}
}

func getUserConnection(users *management.User) []string {
	var res []string
	for _, i := range users.Identities {
		res = append(res, fmt.Sprintf("%v", auth0.StringValue(i.Connection)))

	}
	return res
}

func stringSliceToCommaSeparatedString(s []string) string {
	return strings.Join(s, ", ")
}

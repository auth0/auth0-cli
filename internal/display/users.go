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
}

func (v *userView) AsTableHeader() []string {
	if v.RequireUsername {
		return []string{
			"UserID",
			"Email",
			"Connection",
			"Username",
		}
	}
	return []string{
		"UserID",
		"Email",
		"Connection",
	}
}

func (v *userView) AsTableRow() []string {
	if v.RequireUsername {
		return []string{
			ansi.Faint(v.UserID),
			v.Email,
			v.Connection,
			v.Username,
		}
	}
	return []string{
		ansi.Faint(v.UserID),
		v.Email,
		v.Connection,
	}
}

func (v *userView) KeyValues() [][]string {
	if v.RequireUsername {
		return [][]string{
			[]string{"USERID", ansi.Faint(v.UserID)},
			[]string{"EMAIL", v.Email},
			[]string{"CONNECTION", v.Connection},
			[]string{"USERNAME", v.Username},
		}
	}
	return [][]string{
		[]string{"USERID", ansi.Faint(v.UserID)},
		[]string{"EMAIL", v.Email},
		[]string{"CONNECTION", v.Connection},
	}
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
	for _, c := range users {
		conn := getUserConnection(c)
		res = append(res, &userView{
			UserID:     ansi.Faint(auth0.StringValue(c.ID)),
			Email:      auth0.StringValue(c.Email),
			Connection: stringSliceToCommaSeparatedString(conn),
			Username:   auth0.StringValue(c.Username),
		})
	}

	r.Results(res)
}

func (r *Renderer) UserShow(users *management.User, requireUsername bool) {
	r.Heading("user")

	conn := getUserConnection(users)
	v := &userView{
		RequireUsername: requireUsername,
		UserID:          ansi.Faint(auth0.StringValue(users.ID)),
		Email:           auth0.StringValue(users.Email),
		Connection:      stringSliceToCommaSeparatedString(conn),
		Username:        auth0.StringValue(users.Username),
	}

	r.Result(v)
}

func (r *Renderer) UserCreate(users *management.User, requireUsername bool) {
	r.Heading("user created")

	v := &userView{
		RequireUsername: requireUsername,
		UserID:          ansi.Faint(auth0.StringValue(users.ID)),
		Email:           auth0.StringValue(users.Email),
		Connection:      auth0.StringValue(users.Connection),
		Username:        auth0.StringValue(users.Username),
	}

	r.Result(v)
}

func (r *Renderer) UserUpdate(users *management.User, requireUsername bool) {
	r.Heading("user updated")

	conn := getUserConnection(users)
	v := &userView{
		RequireUsername: requireUsername,
		UserID:          auth0.StringValue(users.ID),
		Email:           auth0.StringValue(users.Email),
		Connection:      stringSliceToCommaSeparatedString(conn),
		Username:        auth0.StringValue(users.Username),
	}

	r.Result(v)
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

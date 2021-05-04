package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type userView struct {
	UserID     string
	Email      string
	Connection string
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
		})
	}

	r.Results(res)
}

func (r *Renderer) UserShow(users *management.User) {
	r.Heading("user")

	conn := getUserConnection(users)

	v := &userView{
		UserID:     ansi.Faint(auth0.StringValue(users.ID)),
		Email:      auth0.StringValue(users.Email),
		Connection: stringSliceToCommaSeparatedString(conn),
	}

	r.Result(v)
}

func (r *Renderer) UserCreate(users *management.User) {
	r.Heading("user created")

	v := &userView{
		UserID:     ansi.Faint(auth0.StringValue(users.ID)),
		Email:      auth0.StringValue(users.Email),
		Connection: auth0.StringValue(users.Connection),
	}

	r.Result(v)
}

func (r *Renderer) UserUpdate(users *management.User) {
	r.Heading("user updated")

	conn := getUserConnection(users)

	v := &userView{
		UserID:     auth0.StringValue(users.ID),
		Email:      auth0.StringValue(users.Email),
		Connection: stringSliceToCommaSeparatedString(conn),
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

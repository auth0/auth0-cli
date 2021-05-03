package display

import (
	"fmt"
	"strings"

	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type userView struct {
	UserID		string
	Name		string
	Username	string
	Email		string
	Connection 	string
}

func (v *userView) AsTableHeader() []string {
	return []string{
		"UserID",
		"Name",
		"Username",
		"Email",
		"Connection",
	}
}

func (v *userView) AsTableRow() []string {
	return []string{
		v.UserID,
		v.Name,
		v.Username,
		v.Email,
		v.Connection,
	}
}

func (v *userView) KeyValues() [][]string {
	return [][]string{
		[]string{"USER ID", v.UserID},
		[]string{"NAME", v.Name},
		[]string{"USERNAME", v.Username},
		[]string{"EMAIL", v.Email},
		[]string{"CONNECTION", v.Connection},
	}
}

type userListView struct {
	UserID		string
	Name		string
	Username	string
	Email		string
	Connection	string
}

func (v *userListView) AsTableHeader() []string {
	return []string{"User ID", "Name", "Username", "Email", "Connection"}
}

func (v *userListView) AsTableRow() []string{
	return []string{
		v.UserID,
		v.Name,
		v.Username,
		v.Email,
		v.Connection,
	}
}

func (r *Renderer) UserList(users []*management.User) {
	resource := "users"

	r.Heading(resource)

	if len(users) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 users create' to add one")
		return
	}

	var res []View
	for _, c := range users {
		conn := getUserConnection(c)
		res = append(res, &userListView{
			UserID: 	auth0.StringValue(c.ID),
			Name:		auth0.StringValue(c.Name),
			Username: 	auth0.StringValue(c.Username),
			Email:		auth0.StringValue(c.Email),
			Connection: stringSliceToCommaSeparatedString(conn),
		})
	}

	r.Results(res)
}

func (r *Renderer) UserShow(users *management.User)  {
	r.Heading("users")

	conn := getUserConnection(users)

	v := &userView{
		UserID:     auth0.StringValue(users.ID),
		Name:       auth0.StringValue(users.Name),
		Username:   auth0.StringValue(users.Username),
		Email:      auth0.StringValue(users.Email),
		Connection: stringSliceToCommaSeparatedString(conn),
	}
	r.Result(v)
}

func (r *Renderer) UserCreate(users *management.User)  {
	r.Heading("users created")

	v := &userView{
		UserID: 	auth0.StringValue(users.ID),
		Connection: auth0.StringValue(users.Connection),
		Name:       auth0.StringValue(users.Name),
		Username:   auth0.StringValue(users.Username),
		Email:      auth0.StringValue(users.Email),
	}

	r.Result(v)
}

func (r *Renderer) UserUpdate(users *management.User)  {
	r.Heading("users updated")

	conn := getUserConnection(users)

	v := &userView{
		UserID: 	auth0.StringValue(users.ID),
		Connection: stringSliceToCommaSeparatedString(conn),
		Name:       auth0.StringValue(users.Name),
		Username:   auth0.StringValue(users.Username),
		Email:      auth0.StringValue(users.Email),
	}

	r.Result(v)
}

func getUserConnection(users *management.User) []string {
	var res []string
	for _, i := range users.Identities{
		res = append(res, fmt.Sprintf("%v", auth0.StringValue(i.Connection)))

	}
	return res
}

func stringSliceToCommaSeparatedString(s []string) string {
	return strings.Join(s, ", ")
}
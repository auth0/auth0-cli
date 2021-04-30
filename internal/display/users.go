package display

import (
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type userView struct {
	UserID		string
	Connection 	string
	Name		string
	Username	string
	Email		string
}

func (v *userView) AsTableHeader() []string {
	return []string{
		"UserID",
		"Connection",
		"Name",
		"Username",
		"Email",
	}
}

func (v *userView) AsTableRow() []string {
	return []string{
		v.UserID,
		v.Connection,
		v.Name,
		v.Username,
		v.Email,
	}
}

func (v *userView) KeyValues() [][]string {
	return [][]string{
		[]string{"USER ID", v.UserID},
		[]string{"CONNECTION", v.Connection},
		[]string{"NAME", v.Name},
		[]string{"USERNAME", v.Username},
		[]string{"EMAIL", v.Email},
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
		res = append(res, &userListView{
			UserID: 	auth0.StringValue(c.ID),
			Name:		auth0.StringValue(c.Name),
			Username: 	auth0.StringValue(c.Username),
			Email:		auth0.StringValue(c.Email),
			Connection: auth0.StringValue(c.Connection),
		})
	}

	r.Results(res)
}

func (r *Renderer) UserShow(users *management.User)  {
	r.Heading("users")

	v := &userView{
		UserID:     auth0.StringValue(users.ID),
		Connection: auth0.StringValue(users.Connection),
		Name:       auth0.StringValue(users.Name),
		Username:   auth0.StringValue(users.Username),
		Email:      auth0.StringValue(users.Email),
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
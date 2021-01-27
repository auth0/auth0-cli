package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type userView struct {
	UserID      string
	Connection  string
	Name        string
	Email       string
	LatestLogin string
}

func (v *userView) AsTableHeader() []string {
	return []string{"User ID", "Name", "Email", "Latest Login"}
}

func (v *userView) AsTableRow() []string {
	return []string{ansi.Faint(v.UserID), v.Name, v.Email, v.LatestLogin}
}

func (r *Renderer) UserList(users []*management.User) {
	r.Heading(ansi.Bold(r.Tenant), "users\n")

	var res []View
	for _, u := range users {
		res = append(res, &userView{
			UserID:      auth0.StringValue(u.ID),
			Name:        auth0.StringValue(u.Name),
			Email:       auth0.StringValue(u.Email),
			LatestLogin: u.LastLogin.String(),
		})
	}

	r.Results(res)
}

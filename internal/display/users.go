package display

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type userView struct {
	UserID          string
	Email           string
	PhoneNumber     string
	Connection      string
	Username        string
	RequireUsername bool
	raw             interface{}
}

func (v *userView) AsTableHeader() []string {
	if v.Connection == management.ConnectionStrategySMS {
		return []string{
			"UserID",
			"PhoneNumber",
			"Connection",
		}
	}

	return []string{
		"UserID",
		"Email",
		"Connection",
	}
}

func (v *userView) AsTableRow() []string {
	if v.Connection == management.ConnectionStrategySMS {
		return []string{
			ansi.Faint(v.UserID),
			v.PhoneNumber,
			v.Connection,
		}
	}

	return []string{
		ansi.Faint(v.UserID),
		v.Email,
		v.Connection,
	}
}

func (v *userView) AsTableRowString() string {
	row := v.AsTableRow()
	return fmt.Sprintf(
		"%-*s  %-*s  %-*s",
		50, row[0],
		50, row[1],
		50, row[2],
	)
}

func (v *userView) AsTableHeaderString() string {
	row := v.AsTableHeader()
	return fmt.Sprintf(
		"    "+"\033[4m%-*s  %-*s  %-*s\033[0m",
		34, row[0],
		50, row[1],
		50, row[2],
	)
}

func (v *userView) KeyValues() [][]string {
	if v.Connection == management.ConnectionStrategySMS {
		return [][]string{
			{"ID", ansi.Faint(v.UserID)},
			{"PHONE-NUMBER", v.PhoneNumber},
			{"CONNECTION", v.Connection},
		}
	} else if v.RequireUsername {
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
		r.EmptyState(resource, "Use 'auth0 users create' to add one")
		return
	}

	var res []View
	for _, user := range users {
		res = append(res, makeUserView(user, false))
	}

	r.Results(res)
}

func (r *Renderer) UserPrompt(users []*management.User, currentIndex *int) string {
	resource := "user"

	r.Heading(resource)

	if len(users) == 0 {
		r.EmptyState(resource, "Use 'auth0 users create' to add one")
		return ""
	}

	label := makeUserView(users[0], false).AsTableHeaderString()
	var rows []string

	// Recursively append each user from users list.
	for _, u := range users {
		rows = append(rows, makeUserView(u, false).AsTableRowString())
	}

	promptui.IconInitial = promptui.Styler()("")
	prompt := promptui.Select{
		Label:    label,
		Items:    rows,
		Size:     10,
		HideHelp: true,
		Stdout:   &noBellStdout{},
		Templates: &promptui.SelectTemplates{
			Label: "{{ . }}",
		},
	}
	var err error
	*currentIndex, _, err = prompt.RunCursorAt(*currentIndex, *currentIndex)
	if err != nil {
		r.Errorf("failed to select a log: %w", err)
	}

	// Return the ID of the select user.
	return users[*currentIndex].GetID()
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
		PhoneNumber:     auth0.StringValue(user.PhoneNumber),
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

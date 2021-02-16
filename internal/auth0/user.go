//go:generate mockgen -source=user.go -destination=user_mock.go -package=auth0

package auth0

import (
	"fmt"

	"gopkg.in/auth0.v5/management"
)

type UserAPI interface {
	// Read a user by its id.
	Read(id string, opts ...management.RequestOption) (u *management.User, err error)

	// List users by email.
	ListByEmail(email string, opts ...management.RequestOption) (us []*management.User, err error)

	// List users.
	List(opts ...management.RequestOption) (ul *management.UserList, err error)
}

// GetUsersForMultiSelect returns a slice of user id and email strings which can be passed into survey.MultiSelect.
func GetUsersForMultiSelect(u UserAPI) ([]string, error) {
	users := []string{}

	list, err := u.List()
	if err != nil {
		return nil, err
	}

	for _, i := range list.Users {
		users = append(users, fmt.Sprintf("%s\t%s", i.GetID(), i.GetEmail()))
	}

	return users, nil
}

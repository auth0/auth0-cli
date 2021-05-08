package auth0

import "github.com/pkg/errors"

func Error(e error, message string) error {
	return errors.Wrap(e, message)
}

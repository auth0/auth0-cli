package ansi

import (
	"errors"
	"github.com/schollz/progressbar/v3"
)

func ProgressBar[T comparable](desc string, items []T, fn func(int, T) error) error {
	switch len(items) {
	case 0:
		return Spinner(desc, func() error {
			return nil
		})
	case 1:
		return Spinner(desc, func() error {
			return fn(1, items[0])
		})
	default:
		bar := progressbar.Default(int64(len(items)), desc)
		var errs []error
		for i, item := range items {
			_ = bar.Add(1)
			if err := fn(i, item); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}

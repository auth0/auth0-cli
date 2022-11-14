package validators

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NoArgs is a validator for commands to print an error when an argument is provided.
func NoArgs(cmd *cobra.Command, args []string) error {
	errorMessage := fmt.Sprintf(
		"`%s` does not take any positional arguments. See `%s --help` for supported flags and usage",
		cmd.CommandPath(),
		cmd.CommandPath(),
	)

	if len(args) > 0 {
		return errors.New(errorMessage)
	}

	return nil
}

// ExactArgs is a validator for commands to print an error when number of
// expected args are different from the number of passed args. The names passed
// in to `expected` are used for the help message.
func ExactArgs(expected ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != len(expected) {
			errorMessage := fmt.Sprintf(
				"`%s` requires %s. See `%s --help` for supported flags and usage",
				cmd.CommandPath(),
				strings.Join(expected, " "),
				cmd.CommandPath(),
			)

			return errors.New(errorMessage)
		}
		return nil
	}
}

// MaximumNArgs is a validator for commands to print an error when the provided
// args are greater than the maximum amount.
func MaximumNArgs(num int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		argument := "positional argument"
		if num > 1 {
			argument = "positional arguments"
		}

		errorMessage := fmt.Sprintf(
			"`%s` accepts at maximum %d %s. See `%s --help` for supported flags and usage",
			cmd.CommandPath(),
			num,
			argument,
			cmd.CommandPath(),
		)

		if len(args) > num {
			return errors.New(errorMessage)
		}
		return nil
	}
}

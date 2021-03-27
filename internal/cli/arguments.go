package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
)

type Argument struct {
	Name string
	Help string
}

func (a Argument) GetName() string {
	return a.Name
}

func (a Argument) GetLabel() string {
	return inputLabel(a.Name)
}

func (a Argument) GetHelp() string {
	return a.Help
}

func (a Argument) GetIsRequired() bool {
	return true
}

func (a *Argument) Ask(cmd *cobra.Command, value interface{}) error {
	return askArgument(cmd, a, value)
}

type pickerOptionsFunc func() (pickerOptions, error)

func (a *Argument) Pick(cmd *cobra.Command, result *string, fn pickerOptionsFunc) error {
	var opts pickerOptions
	err := ansi.Waiting(func() error {
		var err error
		opts, err = fn()
		return err
	})

	if err != nil {
		return err
	}

	defaultLabel := opts.defaultLabel()
	var val string
	if err := selectArgument(cmd, a, &val, opts.labels(), &defaultLabel); err != nil {
		return err
	}

	*result = opts.getValue(val)
	return nil
}

func selectArgument(cmd *cobra.Command, a *Argument, value interface{}, options []string, defaultValue *string) error {
	if canPrompt(cmd) {
		return _select(cmd, a, value, options, defaultValue, false)
	}

	return fmt.Errorf("Missing a required argument: %s", a.GetName())
}

func askArgument(cmd *cobra.Command, i commandInput, value interface{}) error {
	if canPrompt(cmd) {
		return ask(cmd, i, value, nil, false)
	}

	return fmt.Errorf("Missing a required argument: %s", i.GetName())
}

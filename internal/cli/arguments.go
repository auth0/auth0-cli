package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
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

type pickerOptionsFunc func(ctx context.Context) (pickerOptions, error)

func (a *Argument) Pick(cmd *cobra.Command, result *string, fn pickerOptionsFunc) error {
	var opts pickerOptions
	err := ansi.Waiting(func() error {
		var err error
		opts, err = fn(cmd.Context())
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

func (a *Argument) PickMany(cmd *cobra.Command, result *[]string, fn pickerOptionsFunc) error {
	var opts pickerOptions
	err := ansi.Waiting(func() error {
		var err error
		opts, err = fn(cmd.Context())
		return err
	})

	if err != nil {
		return err
	}

	var values []string
	if err := askMultiSelect(a, &values, opts.labels()...); err != nil {
		return err
	}

	*result = opts.getValues(values...)
	return nil
}

func selectArgument(cmd *cobra.Command, a *Argument, value interface{}, options []string, defaultValue *string) error {
	if canPrompt(cmd) {
		return _select(a, value, options, defaultValue, false)
	}

	return fmt.Errorf("Missing a required argument: %s", a.GetName())
}

func askArgument(cmd *cobra.Command, i commandInput, value interface{}) error {
	if canPrompt(cmd) {
		return ask(i, value, nil, false)
	}

	return fmt.Errorf("Missing a required argument: %s", i.GetName())
}

package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type Flag struct {
	Name       string
	LongForm   string
	ShortForm  string
	Help       string
	IsRequired bool
}

func (f *Flag) Ask(cmd *cobra.Command, value interface{}) error {
	return askInput(cmd, f, value, false)
}

func (f *Flag) AskU(cmd *cobra.Command, value interface{}) error {
	return askInput(cmd, f, value, true)
}

func (f *Flag) Select(cmd *cobra.Command, value interface{}, options []string) error {
	return selectInput(cmd, f, value, options, false)
}

func (f *Flag) SelectU(cmd *cobra.Command, value interface{}, options []string) error {
	return selectInput(cmd, f, value, options, true)
}

func (f *Flag) RegisterString(cmd *cobra.Command, value *string, defaultValue string) {
	registerString(cmd, f, value, defaultValue, false)
}

func (f *Flag) RegisterStringU(cmd *cobra.Command, value *string, defaultValue string) {
	registerString(cmd, f, value, defaultValue, true)
}

func (f *Flag) RegisterStringSlice(cmd *cobra.Command, value *[]string, defaultValue []string) {
	registerStringSlice(cmd, f, value, defaultValue, false)
}

func (f *Flag) RegisterStringSliceU(cmd *cobra.Command, value *[]string, defaultValue []string) {
	registerStringSlice(cmd, f, value, defaultValue, true)
}

func (f *Flag) label() string {
	return fmt.Sprintf("%s:", f.Name)
}

func askInput(cmd *cobra.Command, f *Flag, value interface{}, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		input := prompt.TextInput("", f.label(), f.Help, f.IsRequired)

		if err := prompt.AskOne(input, value); err != nil {
			return unexpectedError(err)
		}
	}

	return nil
}

func selectInput(cmd *cobra.Command, f *Flag, value interface{}, options []string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		input := prompt.SelectInput("", f.label(), f.Help, options, f.IsRequired)

		if err := prompt.AskOne(input, value); err != nil {
			return unexpectedError(err)
		}
	}

	return nil
}

func registerString(cmd *cobra.Command, f *Flag, value *string, defaultValue string, isUpdate bool) {
	cmd.Flags().StringVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(unexpectedError(err)) // TODO: Handle
	}
}

func registerStringSlice(cmd *cobra.Command, f *Flag, value *[]string, defaultValue []string, isUpdate bool) {
	cmd.Flags().StringSliceVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(unexpectedError(err)) // TODO: Handle
	}
}

func shouldAsk(cmd *cobra.Command, f *Flag, isUpdate bool) bool {
	if isUpdate {
		return shouldPromptWhenFlagless(cmd, f.LongForm)
	}

	return shouldPrompt(cmd, f.LongForm)
}

func markFlagRequired(cmd *cobra.Command, f *Flag, isUpdate bool) error {
	if f.IsRequired && !isUpdate {
		return cmd.MarkFlagRequired(f.LongForm)
	}

	return nil
}

func unexpectedError(err error) error {
	return fmt.Errorf("An unexpected error occurred: %w", err)
}

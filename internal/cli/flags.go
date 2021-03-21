package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Flag struct {
	Name         string
	LongForm     string
	ShortForm    string
	Help         string
	IsRequired   bool
	AlwaysPrompt bool
}

func (f Flag) GetName() string {
	return f.Name
}

func (f Flag) GetLabel() string {
	return inputLabel(f.Name)
}

func (f Flag) GetHelp() string {
	return f.Help
}

func (f Flag) GetIsRequired() bool {
	return f.IsRequired
}

func (f *Flag) Ask(cmd *cobra.Command, value interface{}) error {
	return askFlag(cmd, f, value, false)
}

func (f *Flag) AskU(cmd *cobra.Command, value interface{}) error {
	return askFlag(cmd, f, value, true)
}

func (f *Flag) AskMany(cmd *cobra.Command, value interface{}) error {
	return askManyFlag(cmd, f, value, false)
}

func (f *Flag) AskManyU(cmd *cobra.Command, value interface{}) error {
	return askManyFlag(cmd, f, value, true)
}

func (f *Flag) Select(cmd *cobra.Command, value interface{}, options []string) error {
	return selectFlag(cmd, f, value, options, false)
}

func (f *Flag) SelectU(cmd *cobra.Command, value interface{}, options []string) error {
	return selectFlag(cmd, f, value, options, true)
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

func (f *Flag) RegisterInt(cmd *cobra.Command, value *int, defaultValue int) {
	registerInt(cmd, f, value, defaultValue, false)
}

func (f *Flag) RegisterIntU(cmd *cobra.Command, value *int, defaultValue int) {
	registerInt(cmd, f, value, defaultValue, true)
}

func (f *Flag) RegisterBool(cmd *cobra.Command, value *bool, defaultValue bool) {
	registerBool(cmd, f, value, defaultValue, false)
}

func (f *Flag) RegisterBoolU(cmd *cobra.Command, value *bool, defaultValue bool) {
	registerBool(cmd, f, value, defaultValue, true)
}

func askFlag(cmd *cobra.Command, f *Flag, value interface{}, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return ask(cmd, f, value, isUpdate)
	}

	return nil
}

func askManyFlag(cmd *cobra.Command, f *Flag, value interface{}, isUpdate bool) error {
	var strInput struct {
		value string
	}

	if err := askFlag(cmd, f, &strInput.value, isUpdate); err != nil {
		return err
	}

	*value.(*[]string) = commaSeparatedStringToSlice(strInput.value)

	return nil
}

func selectFlag(cmd *cobra.Command, f *Flag, value interface{}, options []string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return _select(cmd, f, value, options, isUpdate)
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

func registerInt(cmd *cobra.Command, f *Flag, value *int, defaultValue int, isUpdate bool) {
	cmd.Flags().IntVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(unexpectedError(err)) // TODO: Handle
	}
}

func registerBool(cmd *cobra.Command, f *Flag, value *bool, defaultValue bool, isUpdate bool) {
	cmd.Flags().BoolVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(unexpectedError(err)) // TODO: Handle
	}
}

func shouldAsk(cmd *cobra.Command, f *Flag, isUpdate bool) bool {
	if isUpdate {
		if  !f.IsRequired && !f.AlwaysPrompt {
			return false
		}

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

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
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

func (f *Flag) IsSet(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(f.LongForm)
}

func (f *Flag) Ask(cmd *cobra.Command, value interface{}, defaultValue *string) error {
	return askFlag(cmd, f, value, defaultValue, false)
}

func (f *Flag) AskU(cmd *cobra.Command, value interface{}, defaultValue *string) error {
	return askFlag(cmd, f, value, defaultValue, true)
}

func (f *Flag) AskMany(cmd *cobra.Command, value interface{}, defaultValue *string) error {
	return askManyFlag(cmd, f, value, defaultValue, false)
}

func (f *Flag) AskManyU(cmd *cobra.Command, value interface{}, defaultValue *string) error {
	return askManyFlag(cmd, f, value, defaultValue, true)
}

func (f *Flag) AskBool(cmd *cobra.Command, value *bool, defaultValue *bool) error {
	return askBoolFlag(cmd, f, value, defaultValue, false)
}

func (f *Flag) AskBoolU(cmd *cobra.Command, value *bool, defaultValue *bool) error {
	return askBoolFlag(cmd, f, value, defaultValue, true)
}

func (f *Flag) AskInt(cmd *cobra.Command, value *int, defaultValue *string) error {
	return askIntFlag(cmd, f, value, defaultValue, false)
}

func (f *Flag) AskIntU(cmd *cobra.Command, value *int, defaultValue *string) error {
	return askIntFlag(cmd, f, value, defaultValue, true)
}

func (f *Flag) Select(cmd *cobra.Command, value interface{}, options []string, defaultValue *string) error {
	return selectFlag(cmd, f, value, options, defaultValue, false)
}

func (f *Flag) SelectU(cmd *cobra.Command, value interface{}, options []string, defaultValue *string) error {
	return selectFlag(cmd, f, value, options, defaultValue, true)
}

func (f *Flag) Pick(cmd *cobra.Command, result *string, fn pickerOptionsFunc) error {
	return pickFlag(cmd, f, result, fn, false)
}

func (f *Flag) PickU(cmd *cobra.Command, result *string, fn pickerOptionsFunc) error {
	return pickFlag(cmd, f, result, fn, true)
}

func (f *Flag) OpenEditor(cmd *cobra.Command, value *string, defaultValue, filename string, infoFn func()) error {
	return openEditorFlag(cmd, f, value, defaultValue, filename, infoFn, nil, false)
}

func (f *Flag) OpenEditorW(cmd *cobra.Command, value *string, defaultValue, filename string, infoFn func(), tempFileFn func(string)) error {
	return openEditorFlag(cmd, f, value, defaultValue, filename, infoFn, tempFileFn, false)
}

func (f *Flag) OpenEditorU(cmd *cobra.Command, value *string, defaultValue string, filename string, infoFn func()) error {
	return openEditorFlag(cmd, f, value, defaultValue, filename, nil, nil, true)
}

func (f *Flag) AskPassword(cmd *cobra.Command, value *string, defaultValue *string) error {
	return askPasswordFlag(cmd, f, value, defaultValue, false)
}

func (f *Flag) AskPasswordU(cmd *cobra.Command, value *string, defaultValue *string) error {
	return askPasswordFlag(cmd, f, value, defaultValue, true)
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

func (f *Flag) RegisterStringMap(cmd *cobra.Command, value *map[string]string, defaultValue map[string]string) {
	registerStringMap(cmd, f, value, defaultValue, false)
}

func (f *Flag) RegisterStringMapU(cmd *cobra.Command, value *map[string]string, defaultValue map[string]string) {
	registerStringMap(cmd, f, value, defaultValue, true)
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

func askFlag(cmd *cobra.Command, f *Flag, value interface{}, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return ask(cmd, f, value, defaultValue, isUpdate)
	}

	return nil
}

func askManyFlag(cmd *cobra.Command, f *Flag, value interface{}, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		var strInput string

		if err := ask(cmd, f, &strInput, defaultValue, isUpdate); err != nil {
			return err
		}

		*value.(*[]string) = commaSeparatedStringToSlice(strInput)
	}

	return nil
}

func askBoolFlag(cmd *cobra.Command, f *Flag, value *bool, defaultValue *bool, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		if err := askBool(cmd, f, value, defaultValue); err != nil {
			return err
		}
	}

	return nil
}

func askIntFlag(cmd *cobra.Command, f *Flag, value *int, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return askInt(cmd, f, value, defaultValue, isUpdate)
	}
	return nil
}

func selectFlag(cmd *cobra.Command, f *Flag, value interface{}, options []string, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return _select(cmd, f, value, options, defaultValue, isUpdate)
	}

	return nil
}

func pickFlag(cmd *cobra.Command, f *Flag, result *string, fn pickerOptionsFunc, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
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
		if err := selectFlag(cmd, f, &val, opts.labels(), &defaultLabel, isUpdate); err != nil {
			return err
		}

		*result = opts.getValue(val)
	}

	return nil
}

func askPasswordFlag(cmd *cobra.Command, f *Flag, value *string, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		if err := askPassword(cmd, f, value, defaultValue, isUpdate); err != nil {
			return err
		}
	}

	return nil
}

func openEditorFlag(cmd *cobra.Command, f *Flag, value *string, defaultValue string, filename string, infoFn func(), tempFileFn func(string), isUpdate bool) error {
	if shouldAsk(cmd, f, false) { // Always open the editor on update
		if isUpdate {
			return openUpdateEditor(cmd, f, value, defaultValue, filename)
		} else {
			return openCreateEditor(cmd, f, value, defaultValue, filename, infoFn, tempFileFn)
		}
	}

	return nil
}

func registerString(cmd *cobra.Command, f *Flag, value *string, defaultValue string, isUpdate bool) {
	cmd.Flags().StringVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(auth0.Error(err, "failed to register string flag"))
	}
}

func registerStringSlice(cmd *cobra.Command, f *Flag, value *[]string, defaultValue []string, isUpdate bool) {
	cmd.Flags().StringSliceVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(auth0.Error(err, "failed to register string slice flag"))
	}
}

func registerStringMap(cmd *cobra.Command, f *Flag, value *map[string]string, defaultValue map[string]string, isUpdate bool) {
	cmd.Flags().StringToStringVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(auth0.Error(err, "failed to register string map flag"))
	}
}

func registerInt(cmd *cobra.Command, f *Flag, value *int, defaultValue int, isUpdate bool) {
	cmd.Flags().IntVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(auth0.Error(err, "failed to register int flag"))
	}
}

func registerBool(cmd *cobra.Command, f *Flag, value *bool, defaultValue bool, isUpdate bool) {
	cmd.Flags().BoolVarP(value, f.LongForm, f.ShortForm, defaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(auth0.Error(err, "failed to register bool flag"))
	}
}

func shouldAsk(cmd *cobra.Command, f *Flag, isUpdate bool) bool {
	if isUpdate {
		if !f.IsRequired && !f.AlwaysPrompt {
			return false
		}
		return shouldPromptWhenFlagless(cmd, f.LongForm)
	}

	return shouldPrompt(cmd, f)
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

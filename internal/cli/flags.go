package cli

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
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

func (f *Flag) Select(cmd *cobra.Command, value interface{}, options []string, defaultValue *string) error {
	return selectFlag(cmd, f, value, options, defaultValue, false)
}

func (f *Flag) SelectU(cmd *cobra.Command, value interface{}, options []string, defaultValue *string) error {
	return selectFlag(cmd, f, value, options, defaultValue, true)
}

func (f *Flag) EditorPrompt(cmd *cobra.Command, value *string, initialValue, filename string, infoFn func()) error {
	out, err := prompt.CaptureInputViaEditor(
		initialValue,
		filename,
		infoFn,
		nil,
	)
	if err != nil {
		return err
	}

	*value = out
	return nil
}

func (f *Flag) EditorPromptW(cmd *cobra.Command, value *string, initialValue, filename string, infoFn func(), tempFileCreatedFn func(string)) error {
	out, err := prompt.CaptureInputViaEditor(
		initialValue,
		filename,
		infoFn,
		tempFileCreatedFn,
	)
	if err != nil {
		return err
	}

	*value = out
	return nil
}

func (f *Flag) EditorPromptU(cmd *cobra.Command, value *string, initialValue, filename string, infoFn func()) error {
	response := map[string]interface{}{}

	questions := []*survey.Question{
		{
			Name: f.Name,
			Prompt: &prompt.Editor{
				BlankAllowed: true,
				Editor: &survey.Editor{
					Help:          f.Help,
					Message:       f.Name,
					FileName:      filename,
					Default:       initialValue,
					HideDefault:   true,
					AppendDefault: true,
				},
			},
		},
	}

	if err := survey.Ask(questions, &response, prompt.Icons); err != nil {
		return err
	}

	// Since we have BlankAllowed=true, an empty answer means we'll use the
	// initialValue provided since this path is for the Update path.
	answer, ok := response[f.Name].(string)
	if ok && answer != "" {
		*value = answer
	} else {
		*value = initialValue
	}

	return nil
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
	var strInput string

	if err := askFlag(cmd, f, &strInput, defaultValue, isUpdate); err != nil {
		return err
	}

	*value.(*[]string) = commaSeparatedStringToSlice(strInput)

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

func selectFlag(cmd *cobra.Command, f *Flag, value interface{}, options []string, defaultValue *string, isUpdate bool) error {
	if shouldAsk(cmd, f, isUpdate) {
		return _select(cmd, f, value, options, defaultValue, isUpdate)
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

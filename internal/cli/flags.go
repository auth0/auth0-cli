package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type Flag struct {
	Name          string
	LongForm      string
	ShortForm     string
	DefaultValue  string
	Help          string
	IsRequired    bool
}

func (f *Flag) Ask(cmd *cobra.Command, value interface{}) error {
	return ask(cmd, f, value, false)
}

func (f *Flag) AskU(cmd *cobra.Command, value interface{}) error {
	return ask(cmd, f, value, true)
}

func (f *Flag) RegisterString(cmd *cobra.Command, value *string) {
	registerString(cmd, f, value, false)
}

func (f *Flag) RegisterStringU(cmd *cobra.Command, value *string) {
	registerString(cmd, f, value, true)
}

func ask(cmd *cobra.Command, f *Flag, value interface{}, isUpdate bool) error {
	var shouldAsk bool

	if isUpdate {
		shouldAsk = shouldPromptWhenFlagless(cmd, f.LongForm)
	} else {
		shouldAsk = shouldPrompt(cmd, f.LongForm)
	}

	if shouldAsk {
		input := prompt.TextInput("", fmt.Sprintf("%s:", f.Name), f.Help, f.IsRequired)

		if err := prompt.AskOne(input, value); err != nil {
			return fmt.Errorf("An unexpected error occurred: %w", err)
		}
	}

	return nil
}

func registerString(cmd *cobra.Command, f *Flag, value *string, isUpdate bool) {
	cmd.Flags().StringVarP(value, f.LongForm, f.ShortForm, f.DefaultValue, f.Help)

	if err := markFlagRequired(cmd, f, isUpdate); err != nil {
		panic(fmt.Errorf("An unexpected error occurred: %w", err)) // TODO: Handle
	}
}

func markFlagRequired(cmd *cobra.Command, f *Flag, isUpdate bool) error {
	if f.IsRequired && !isUpdate {
		return cmd.MarkFlagRequired(f.LongForm)
	}

	return nil
}

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

func (f *Flag) AskUpdate(cmd *cobra.Command, value interface{}) error {
	return ask(cmd, f, value, true)
}

func (f *Flag) RegisterString(cmd *cobra.Command, value *string) {
	cmd.Flags().StringVarP(value, f.LongForm, f.ShortForm, f.DefaultValue, f.Help)
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

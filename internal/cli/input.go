package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type CommandInput interface {
	GetName() string
	GetLabel() string
	GetHelp() string
	GetIsInputRequired() bool
}

func ask(cmd *cobra.Command, i CommandInput, value interface{}, isUpdate bool) error {
	isRequired := !isUpdate && i.GetIsInputRequired()
	input := prompt.TextInput("", i.GetLabel(), i.GetHelp(), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return unexpectedError(err)
	}

	return nil
}

func _select(cmd *cobra.Command, i CommandInput, value interface{}, options []string, isUpdate bool) error {
	isRequired := !isUpdate && i.GetIsInputRequired()
	input := prompt.SelectInput("", i.GetLabel(), i.GetHelp(), options, isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return unexpectedError(err)
	}

	return nil
}

func inputLabel(name string) string {
	return fmt.Sprintf("%s:", name)
}

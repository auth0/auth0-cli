package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type commandInput interface {
	GetName() string
	GetLabel() string
	GetHelp() string
	GetIsRequired() bool
}

func ask(cmd *cobra.Command, i commandInput, value interface{}, isUpdate bool) error {
	isRequired := !isUpdate && i.GetIsRequired()
	input := prompt.TextInput("", i.GetLabel(), i.GetHelp(), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return unexpectedError(err)
	}

	return nil
}

func _select(cmd *cobra.Command, i commandInput, value interface{}, options []string, isUpdate bool) error {
	isRequired := !isUpdate && i.GetIsRequired()
	input := prompt.SelectInput("", i.GetLabel(), i.GetHelp(), options, isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return unexpectedError(err)
	}

	return nil
}

func inputLabel(name string) string {
	return fmt.Sprintf("%s:", name)
}

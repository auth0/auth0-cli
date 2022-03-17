package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/auth0/go-auth0"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
)

type commandInput interface {
	GetName() string
	GetLabel() string
	GetHelp() string
	GetIsRequired() bool
}

func ask(cmd *cobra.Command, i commandInput, value interface{}, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.TextInput("", i.GetLabel(), i.GetHelp(), auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askBool(cmd *cobra.Command, i commandInput, value *bool, defaultValue *bool) error {
	if err := prompt.AskBool(i.GetLabel(), value, auth0.BoolValue(defaultValue)); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askInt(cmd *cobra.Command, i commandInput, value *int, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.TextInput(i.GetName(), i.GetLabel(), i.GetHelp(), auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askPassword(cmd *cobra.Command, i commandInput, value interface{}, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.PasswordInput("", i.GetLabel(), auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func _select(cmd *cobra.Command, i commandInput, value interface{}, options []string, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)

	// If there is no provided default value, we'll use the first option in
	// the selector by default.
	if defaultValue == nil && len(options) > 0 {
		defaultValue = &(options[0])
	}

	input := prompt.SelectInput("", i.GetLabel(), i.GetHelp(), options, auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func openCreateEditor(cmd *cobra.Command, i commandInput, value *string, defaultValue string, filename string, infoFn func(), tempFileFn func(string)) error {
	out, err := prompt.CaptureInputViaEditor(
		defaultValue,
		filename,
		infoFn,
		tempFileFn,
	)

	if err != nil {
		return handleInputError(err)
	}

	*value = out

	return nil
}

func openUpdateEditor(cmd *cobra.Command, i commandInput, value *string, defaultValue string, filename string) error {
	isRequired := isInputRequired(i, true)
	response := map[string]interface{}{}
	input := prompt.EditorInput(i.GetName(), i.GetLabel(), i.GetHelp(), filename, defaultValue, isRequired)

	if err := prompt.AskOne(input, &response); err != nil {
		return handleInputError(err)
	}

	// Since we have BlankAllowed=true, an empty answer means we'll use the
	// initialValue provided since this path is for the Update path.
	answer, ok := response[i.GetName()].(string)
	if ok && answer != "" {
		*value = answer
	} else {
		*value = defaultValue
	}

	return nil
}

func inputLabel(name string) string {
	return fmt.Sprintf("%s:", name)
}

func isInputRequired(i commandInput, isUpdate bool) bool {
	return !isUpdate && i.GetIsRequired()
}

func handleInputError(err error) error {
	if err == terminal.InterruptErr {
		os.Exit(0)
	}

	return unexpectedError(err)
}

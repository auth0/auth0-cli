package cli

import (
	"fmt"
	"os"
	"reflect"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/auth0/go-auth0"

	"github.com/auth0/auth0-cli/internal/prompt"
)

type commandInput interface {
	GetName() string
	GetLabel() string
	GetHelp() string
	GetIsRequired() bool
}

func ask(i commandInput, value interface{}, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.TextInput("", i.GetLabel(), i.GetHelp(), auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askBool(i commandInput, value *bool, defaultValue *bool) error {
	if err := prompt.AskBool(i.GetLabel(), value, auth0.BoolValue(defaultValue)); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askInt(i commandInput, value *int, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.TextInput(i.GetName(), i.GetLabel(), i.GetHelp(), auth0.StringValue(defaultValue), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askPassword(i commandInput, value interface{}, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)
	input := prompt.PasswordInput("", i.GetLabel(), isRequired)

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func askMultiSelect(i commandInput, value interface{}, options ...string) error {
	v := reflect.ValueOf(options)
	if v.Kind() != reflect.Slice || v.Len() <= 0 {
		return handleInputError(fmt.Errorf("there is not enough data to select from"))
	}
	if err := prompt.AskMultiSelect(i.GetLabel(), value, options...); err != nil {
		return handleInputError(err)
	}
	return nil
}

func _select(i commandInput, value interface{}, options []string, defaultValue *string, isUpdate bool) error {
	isRequired := isInputRequired(i, isUpdate)

	// If there is no provided default value, we'll use the first option in
	// the selector by default.
	if defaultValue == nil && len(options) > 0 {
		defaultValue = &(options[0])
	}

	var input *survey.Question

	// Use paginated select for large option sets (>15 options).
	if len(options) > 15 {
		input = prompt.PaginatedSelectInput("", i.GetLabel(), i.GetHelp(), options, auth0.StringValue(defaultValue), isRequired)
	} else {
		input = prompt.SelectInput("", i.GetLabel(), i.GetHelp(), options, auth0.StringValue(defaultValue), isRequired)
	}

	if err := prompt.AskOne(input, value); err != nil {
		return handleInputError(err)
	}

	return nil
}

func openCreateEditor(value *string, defaultValue string, filename string, infoFn func(), tempFileFn func(string)) error {
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

func openUpdateEditor(i commandInput, value *string, defaultValue string, filename string) error {
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

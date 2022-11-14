package prompt

import (
	"github.com/AlecAivazis/survey/v2"

	"github.com/auth0/auth0-cli/internal/iostream"
)

var stdErrWriter = survey.WithStdio(iostream.Input, iostream.Messages, iostream.Messages)

var Icons = survey.WithIcons(func(icons *survey.IconSet) {
	icons.Question.Text = ""
})

func Ask(inputs []*survey.Question, response interface{}) error {
	return survey.Ask(inputs, response, stdErrWriter, Icons)
}

func AskOne(input *survey.Question, response interface{}) error {
	return survey.Ask([]*survey.Question{input}, response, stdErrWriter, Icons)
}

func askOne(prompt survey.Prompt, response interface{}) error {
	return survey.AskOne(prompt, response, stdErrWriter, Icons)
}

func AskBool(message string, value *bool, defaultValue bool) error {
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue,
	}

	if err := askOne(prompt, value); err != nil {
		return err
	}

	return nil
}

func Confirm(message string) bool {
	result := false
	prompt := &survey.Confirm{
		Message: message,
	}

	if err := askOne(prompt, &result); err != nil {
		return false
	}

	return result
}

func TextInput(name string, message string, help string, defaultValue string, required bool) *survey.Question {
	input := &survey.Question{
		Name:   name,
		Prompt: &survey.Input{Message: message, Help: help, Default: defaultValue},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

func BoolInput(name string, message string, help string, defaultValue bool, required bool) *survey.Question {
	input := &survey.Question{
		Name:      name,
		Prompt:    &survey.Confirm{Message: message, Help: help, Default: defaultValue},
		Transform: survey.Title,
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

func SelectInput(name string, message string, help string, options []string, defaultValue string, required bool) *survey.Question {
	// force options "page" size to full,
	// since there's not visual clue about extra options.
	pageSize := len(options)
	input := &survey.Question{
		Name:   name,
		Prompt: &survey.Select{Message: message, Help: help, Options: options, PageSize: pageSize, Default: defaultValue},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

func PasswordInput(name string, message string, defaultValue string, required bool) *survey.Question {
	input := &survey.Question{
		Name:   name,
		Prompt: &survey.Password{Message: message},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

func EditorInput(name string, message string, help string, filename string, defaultValue string, required bool) *survey.Question {
	input := &survey.Question{
		Name: name,
		Prompt: &Editor{
			BlankAllowed: true,
			Editor: &survey.Editor{
				Help:          help,
				Message:       message,
				FileName:      filename,
				Default:       defaultValue,
				HideDefault:   true,
				AppendDefault: true,
			},
		},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

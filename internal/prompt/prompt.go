package prompt

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
)

var stdErrWriter = survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)

var icons = survey.WithIcons(func(icons *survey.IconSet) {
	icons.Question.Text = ""
})

func Ask(inputs []*survey.Question, response interface{}) error {
	return survey.Ask(inputs, response, stdErrWriter, icons)
}

func AskOne(input *survey.Question, response interface{}) error {
	return survey.Ask([]*survey.Question{input}, response, stdErrWriter, icons)
}

func askOne(prompt survey.Prompt, response interface{}) error {
	return survey.AskOne(prompt, response, stdErrWriter, icons)
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

func BoolInput(name string, message string, help string, required bool) *survey.Question {
	input := &survey.Question{
		Name:      name,
		Prompt:    &survey.Confirm{Message: message, Help: help},
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
		Prompt: &survey.Select{Message: message, Help: help, Options: options, PageSize: pageSize, Default: defaultValue},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
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

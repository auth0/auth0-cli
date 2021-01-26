package prompt

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
)

var stdErrWriter = survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)

func Ask(inputs []*survey.Question, response interface{}) error {
	return survey.Ask(inputs, response, stdErrWriter)
}

func TextInput(name string, message string, help string, required bool) *survey.Question {
	input := &survey.Question{
		Name:      name,
		Prompt:    &survey.Input{Message: message, Help: help},
		Transform: survey.Title,
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

	if err := survey.AskOne(prompt, &result, stdErrWriter); err != nil {
		return false
	}

	return result
}

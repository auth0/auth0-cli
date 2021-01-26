package prompt

import (
	"github.com/AlecAivazis/survey/v2"
)

func Ask(inputs []*survey.Question, response interface{}, options ...survey.AskOpt) error {
	return survey.Ask(inputs, response, options...)
}

func TextInput(name string, message string, required bool) *survey.Question {
	input := &survey.Question{
        Name:     name,
        Prompt:   &survey.Input{Message: message},
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

	survey.AskOne(prompt, &result)

	return result
}

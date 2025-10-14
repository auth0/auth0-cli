package prompt

import (
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

var (
	defaultPageSize = 13
)

func PaginatedSelectInput(name string, message string, help string, options []string, defaultValue string, required bool) *survey.Question {
	input := &survey.Question{
		Name: name,
		Prompt: &survey.Select{
			Message:  message,
			Help:     help,
			Options:  options,
			PageSize: defaultPageSize,
			Default:  defaultValue,

			Filter: func(filterVal string, optionVal string, optionIndex int) bool {
				// Case-insensitive search - matches if the search term appears anywhere in the option.
				return strings.Contains(strings.ToLower(optionVal), strings.ToLower(filterVal))
			},
		},
	}

	if required {
		input.Validate = survey.Required
	}

	return input
}

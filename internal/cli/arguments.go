package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Argument struct {
	Name       string
	Help       string
	IsRequired bool
}

func (a Argument) GetName() string {
	return a.Name
}

func (a Argument) GetLabel() string {
	return inputLabel(a.Name)
}

func (a Argument) GetHelp() string {
	return a.Help
}

func (a Argument) GetIsInputRequired() bool {
	return a.IsRequired
}

func (a *Argument) Ask(cmd *cobra.Command, value interface{}) error {
	return askArgument(cmd, a, value)
}

func askArgument(cmd *cobra.Command, i CommandInput, value interface{}) error {
	if canPrompt(cmd) {
		return ask(cmd, i, value, true)
	} else {
		return fmt.Errorf("Missing a required argument: %s", i.GetName())
	}
}

package iostream

import (
	"bufio"
	"io"
	"os"

	"github.com/mattn/go-isatty"

	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	Input    = os.Stdin
	Output   = os.Stdout
	Messages = os.Stderr
)

func IsInputTerminal() bool {
	return isatty.IsTerminal(Input.Fd())
}

func IsOutputTerminal() bool {
	return isatty.IsTerminal(Output.Fd())
}

func PipedInput() []byte {
	if !IsInputTerminal() {
		reader := bufio.NewReader(Input)
		var pipedInput []byte

		for {
			input, err := reader.ReadBytes('\n')
			if err == io.EOF {
				// Handle the last partial line (if any) at EOF.
				if len(input) > 0 {
					pipedInput = append(pipedInput, input...)
				}
				break
			} else if err != nil {
				panic(auth0.Error(err, "unable to read from pipe"))
			}
			pipedInput = append(pipedInput, input...)
		}

		return pipedInput
	}

	return []byte{}
}

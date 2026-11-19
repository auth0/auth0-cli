package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// newTestPromptScreen starts a screen that writes to a buffer, the title of the
// run and the blocks both, and asks with the asker given. An interrupt ends the command with the error itself
// instead of the process.
func newTestPromptScreen(t *testing.T, asker promptAsker) (*questionScreen, *bytes.Buffer) {
	t.Helper()

	blocks := &bytes.Buffer{}
	renderer := &display.Renderer{ResultWriter: blocks, MessageWriter: blocks}

	if asker.interrupt == nil {
		asker.interrupt = func(err error) error { return err }
	}

	return &questionScreen{
		PromptScreen: renderer.NewPromptScreen("Poppy setup", "mock.auth0.test"),
		asker:        asker,
	}, blocks
}

// poppyScript answers the prompts of a test in order and keeps what they asked.
// An answer is a bool for a question of yes or no, an int for a list, an error
// for Ctrl-C, and nil for Enter, which takes the default.
type poppyScript struct {
	answers []any
	asked   []string
}

func (s *poppyScript) next(t *testing.T, message string) any {
	t.Helper()

	s.asked = append(s.asked, message)
	require.NotEmpty(t, s.answers, "no answer was scripted for %q", message)

	answer := s.answers[0]
	s.answers = s.answers[1:]

	return answer
}

func (s *poppyScript) asker(t *testing.T) promptAsker {
	return promptAsker{
		confirm: func(message string, def bool) (bool, error) {
			switch answer := s.next(t, message).(type) {
			case nil:
				return def, nil
			case bool:
				return answer, nil
			case error:
				return false, answer
			default:
				require.Failf(t, "wrong answer", "%T for a question of yes or no", answer)

				return false, nil
			}
		},
		choose: func(message string, _ []string, def int) (int, error) {
			switch answer := s.next(t, message).(type) {
			case nil:
				return def, nil
			case int:
				return answer, nil
			case error:
				return 0, answer
			default:
				require.Failf(t, "wrong answer", "%T for a list", answer)

				return 0, nil
			}
		},
	}
}

func TestPromptScreenAskLine(t *testing.T) {
	t.Run("an empty answer asks again, and the error goes with the block", func(t *testing.T) {
		script := []string{"", "  ", "https://api.shop.example"}
		screen, out := newTestPromptScreen(t, promptAsker{
			text: func(string, string) (string, error) {
				answer := script[0]
				script = script[1:]

				return answer, nil
			},
		})

		answer, err := screen.askLine("API", "The API.", "API", "")

		require.NoError(t, err)
		assert.Equal(t, "https://api.shop.example", answer)
		assert.Equal(t, 2, strings.Count(out.String(), "A value is required."))
		assert.Contains(t, out.String(), "API\nThe API.\n", "the block stays on the screen")
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		screen, out := newTestPromptScreen(t, promptAsker{
			text: func(string, string) (string, error) { return "", terminal.InterruptErr },
		})

		_, err := screen.askLine("API", "The API.", "API", "")

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.True(t, strings.HasSuffix(out.String(), display.PromptCancelled+"\n"))
	})

	t.Run("an answer the question refuses ends the command", func(t *testing.T) {
		screen, _ := newTestPromptScreen(t, promptAsker{
			text: func(string, string) (string, error) { return "https://shop.example/path", nil },
		})

		var in setup.Inputs

		err := screen.askText(setup.RequiredQuestions(in)[0], &in)

		require.Error(t, err)
		assert.Equal(t, "invalid_flag", errorReason(err))
		assert.Empty(t, in.CompanyDomain)
	})

	t.Run("an answer the question takes is stored", func(t *testing.T) {
		screen, _ := newTestPromptScreen(t, promptAsker{
			text: func(string, string) (string, error) { return "Shop.Example", nil },
		})

		var in setup.Inputs

		require.NoError(t, screen.askText(setup.RequiredQuestions(in)[0], &in))
		assert.Equal(t, "shop.example", in.CompanyDomain)
	})
}

func TestPromptScreenConfirm(t *testing.T) {
	tests := []struct {
		name   string
		answer bool
		def    bool
	}{
		{name: "yes", answer: true},
		{name: "no", answer: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotDefault bool

			screen, out := newTestPromptScreen(t, promptAsker{
				confirm: func(_ string, def bool) (bool, error) {
					gotDefault = def

					return test.answer, nil
				},
			})

			yes, err := screen.confirmAsk("Apply the plan", "It changes your tenant.", "Apply 3 changes?")

			require.NoError(t, err)
			assert.Equal(t, test.answer, yes)
			assert.False(t, gotDefault, "Enter is no")
			assert.Contains(t, out.String(), "Apply the plan\nIt changes your tenant.\n")
		})
	}
}

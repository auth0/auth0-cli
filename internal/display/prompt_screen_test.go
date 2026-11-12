package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// newTestPromptScreen starts a screen of a renderer that writes to two buffers,
// the result writer and the message writer.
func newTestPromptScreen(format OutputFormat) (screen *PromptScreen, result, messages *bytes.Buffer) {
	result, messages = &bytes.Buffer{}, &bytes.Buffer{}

	r := &Renderer{ResultWriter: result, MessageWriter: messages, Format: format}

	return r.NewPromptScreen("Poppy setup", "mock.auth0.test"), result, messages
}

func TestPromptScreenTitle(t *testing.T) {
	ask := func(s *PromptScreen) { s.Begin("Question", "Why.") }

	t.Run("the title of the run is printed once, above the first question", func(t *testing.T) {
		screen, result, messages := newTestPromptScreen("")
		screen.ForDomain("shop.example")

		ask(screen)
		ask(screen)

		assert.Equal(t, 1, strings.Count(result.String(), "Poppy setup for shop.example"))
		assert.True(t, strings.HasPrefix(result.String(), "Poppy setup for shop.example"))
		assert.True(t, screen.HeaderShown())
		assert.Equal(t, 2, strings.Count(messages.String(), "Question\nWhy.\n"))
	})

	t.Run("before the company domain is known the title is printed once at the top, without it", func(t *testing.T) {
		screen, result, messages := newTestPromptScreen("")

		ask(screen)
		screen.ForDomain("shop.example")
		ask(screen)

		assert.Equal(t, 1, strings.Count(result.String(), "Poppy setup"))
		assert.True(t, strings.HasPrefix(result.String(), "Poppy setup"))
		assert.NotContains(t, result.String(), "shop.example")
		assert.True(t, screen.HeaderShown(), "the text that follows does not print it again")
		assert.Equal(t, 2, strings.Count(messages.String(), "Question\nWhy.\n"))
	})

	t.Run("the title is not printed again after text that carries it", func(t *testing.T) {
		screen, result, _ := newTestPromptScreen("")
		screen.ForDomain("shop.example")

		screen.Printed()
		ask(screen)

		assert.Empty(t, result.String())
		assert.True(t, screen.HeaderShown())
	})

	t.Run("a block stays: nothing is erased", func(t *testing.T) {
		screen, result, messages := newTestPromptScreen("")
		screen.ForDomain("shop.example")

		ask(screen)
		screen.Note("Nothing to %s.", "choose")
		screen.Invalid("A value is required.")

		assert.NotContains(t, result.String()+messages.String(), "\x1b[J")
		assert.Contains(t, messages.String(), "Nothing to choose.")
		assert.Contains(t, messages.String(), "A value is required.")
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		screen, result, messages := newTestPromptScreen("")

		screen.Cancelled()

		assert.Empty(t, result.String())
		assert.Equal(t, PromptCancelled+"\n", messages.String())
	})
}

func TestPromptScreenStreams(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
		// Whether the title of the run is on stdout, else it is on stderr.
		titleOnStdout bool
	}{
		{name: "text", titleOnStdout: true},
		{name: "json", format: OutputFormatJSON},
		{name: "compact json", format: OutputFormatJSONCompact},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			screen, result, messages := newTestPromptScreen(test.format)
			screen.ForDomain("shop.example")

			screen.Begin("Question", "Why.")
			screen.Note("A note.")
			screen.Invalid("Not valid.")
			screen.Cancelled()

			// The blocks are on stderr whatever the format.
			for _, want := range []string{"Question\nWhy.\n", "A note.", "Not valid.", PromptCancelled} {
				assert.Contains(t, messages.String(), want)
			}

			title := "Poppy setup for shop.example"
			if test.titleOnStdout {
				assert.Contains(t, result.String(), title)
				assert.NotContains(t, messages.String(), title)

				return
			}

			assert.Empty(t, result.String(), "only the JSON goes to stdout")
			assert.Contains(t, messages.String(), title)
		})
	}
}

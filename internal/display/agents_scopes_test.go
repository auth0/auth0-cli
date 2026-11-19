package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestPoppyHintNote(t *testing.T) {
	tests := []struct {
		name  string
		hints *config.Hints
		want  string
	}{
		{name: "no hints", want: ""},
		{name: "changes nothing", hints: &config.Hints{Changes: "none"}, want: "[changes: none]"},
		{name: "personal data", hints: &config.Hints{Changes: "reversible", PersonalData: true}, want: "[changes: reversible, personal data]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, poppyHintNote(test.hints))
		})
	}
}

func TestScopeRows(t *testing.T) {
	scopes := []ScopeChoice{
		{Name: "orders:read", Description: "See your orders and their status", Hints: &config.Hints{Changes: "none"}},
		{Name: "checkout", Description: "Place orders and pay with your saved card"},
		{Name: "poppy:browser-sign-in", Description: "Use the website signed in, with the other access you approve"},
	}

	rowsAt := func(width int) []string {
		var out bytes.Buffer

		r := &Renderer{ResultWriter: &out, MessageWriter: &out}

		screen := r.NewPromptScreen("", "")
		screen.layout = Layout{Width: width, Unicode: true}

		return screen.ScopeRows(scopes)
	}

	t.Run("names are aligned and the descriptions follow", func(t *testing.T) {
		assert.Equal(t, []string{
			"orders:read            See your orders and their status [changes: none]",
			"checkout               Place orders and pay with your saved card",
			"poppy:browser-sign-in  Use the website signed in, with the other access you approve",
		}, rowsAt(120))
	})

	t.Run("a description that does not fit is cut, never wrapped", func(t *testing.T) {
		for _, row := range rowsAt(60) {
			assert.LessOrEqual(t, len([]rune(row))+ScopeListMarks, 60, row)
		}

		assert.Contains(t, rowsAt(60)[2], "…")
	})

	t.Run("the hints stay when the description is cut", func(t *testing.T) {
		assert.True(t, strings.HasSuffix(rowsAt(60)[0], "[changes: none]"), rowsAt(60)[0])
	})
}

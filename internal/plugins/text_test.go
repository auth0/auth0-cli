package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "keeps plain text", text: "Run a tenant audit, v1.0", want: "Run a tenant audit, v1.0"},
		{name: "keeps non-ASCII letters", text: "Zoë Müller 日本語", want: "Zoë Müller 日本語"},
		{name: "keeps an empty string", text: "", want: ""},
		{name: "replaces a control character (Cc)", text: "a\x1b[31mb", want: "a\ufffd[31mb"},
		{name: "replaces a newline (Cc)", text: "a\nb", want: "a\ufffdb"},
		{name: "replaces a bidirectional override (Cf)", text: "a\u202eb", want: "a\ufffdb"},
		{name: "replaces a zero-width space (Cf)", text: "a\u200bb", want: "a\ufffdb"},
		{name: "replaces a line separator (Zl)", text: "a\u2028b", want: "a\ufffdb"},
		{name: "replaces a paragraph separator (Zp)", text: "a\u2029b", want: "a\ufffdb"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, SafeText(test.text))
		})
	}
}

package text

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestEscape(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text is unchanged", in: "Shop Web, café ✓ 日本語", want: "Shop Web, café ✓ 日本語"},
		{name: "empty", in: "", want: ""},
		{name: "escape sequence", in: "\x1b[31mred\x1b[0m", want: `\x1b[31mred\x1b[0m`},
		{name: "carriage return and tab", in: "a\rb\tc", want: `a\x0db\x09c`},
		{name: "delete and NUL", in: "a\x7fb\x00c", want: `a\x7fb\x00c`},
		{name: "C1 control", in: "a\u009bb", want: `a\x9bb`},
		{name: "newline is kept", in: "a\nb", want: "a\nb"},
		{name: "invalid UTF-8", in: "a\xffb", want: `a\xffb`},
		{name: "right-to-left override", in: "gpj.exe\u202e", want: `gpj.exe\u202e`},
		{name: "isolate and mark", in: "a\u2066b\u200fc", want: `a\u2066b\u200fc`},
		{name: "Arabic letter mark", in: "a\u061cb", want: `a\u061cb`},
		{name: "emoji joiner is kept", in: "👩\u200d💻", want: "👩\u200d💻"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Escape(test.in))
		})
	}
}

func TestLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text is unchanged", in: "Shop Web ✓", want: "Shop Web ✓"},
		{name: "newline becomes a space", in: "a\nb", want: "a b"},
		{name: "a run of newlines becomes one space", in: "a\n\n\nb", want: "a b"},
		{name: "forged line", in: "ok\n✓    D9 everything is fine", want: "ok ✓    D9 everything is fine"},
		{name: "escape sequence", in: "\x1b[2J", want: `\x1b[2J`},
		{name: "carriage return", in: "a\r\nb", want: `a\x0d b`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Line(test.in))
		})
	}
}

func TestCap(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{name: "short text is unchanged", in: "hello", limit: 5, want: "hello"},
		{name: "long text is cut with an ellipsis", in: "hello world", limit: 6, want: "hello…"},
		{name: "counts runes, not bytes", in: "日本語日本語", limit: 4, want: "日本語…"},
		{name: "no room", in: "hello", limit: 0, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Cap(test.in, test.limit))
		})
	}

	t.Run("one mebibyte", func(t *testing.T) {
		got := Cap(Line(strings.Repeat("é", 1<<20)), 200)

		assert.Equal(t, 200, utf8.RuneCountInString(got))
		assert.True(t, strings.HasSuffix(got, Ellipsis))
	})
}

func TestQuote(t *testing.T) {
	tests := map[string]string{
		"shop.example":        "shop.example",
		"Shop Web":            `"Shop Web"`,
		"":                    `""`,
		`it's $5`:             `'it'\''s $5'`,
		"https://a.b/c?d=e&f": `"https://a.b/c?d=e&f"`,
	}

	for in, want := range tests {
		assert.Equal(t, want, Quote(in), in)
	}
}

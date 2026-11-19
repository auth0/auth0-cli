// Package text makes strings from third parties safe to print in a terminal, and
// quotes them for a shell.
package text

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ellipsis ends a string that Cap cut short.
const Ellipsis = "…"

// Escape replaces every control character, and every byte that is not valid
// UTF-8, with a visible \xNN or \uNNNN escape, so that the text cannot move the
// cursor, restyle the terminal or forge a line. Newlines are kept.
func Escape(s string) string {
	return escape(s, false)
}

// Line is Escape for text that must stay on one line: each run of newlines
// becomes a single space.
func Line(s string) string {
	return escape(s, true)
}

// Cap cuts s to at most limit runes, ending it with an ellipsis when it cut
// anything. It is meant for text that Line has already cleaned.
func Cap(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	if len(s) <= limit || utf8.RuneCountInString(s) <= limit {
		return s
	}

	runes := []rune(s[:min(len(s), limit*utf8.UTFMax)])

	return string(runes[:limit-1]) + Ellipsis
}

func escape(s string, singleLine bool) string {
	var b strings.Builder
	b.Grow(len(s))

	inNewlines := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])

		switch {
		case r == '\n':
			if !singleLine {
				b.WriteByte('\n')
			} else if !inNewlines {
				b.WriteByte(' ')
			}
			inNewlines = true
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
			inNewlines = false
		case unicode.IsControl(r) || isBidiControl(r):
			if r < 0x100 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				fmt.Fprintf(&b, `\u%04x`, r)
			}
			inNewlines = false
		default:
			b.WriteString(s[i : i+size])
			inNewlines = false
		}

		i += size
	}

	return b.String()
}

// isBidiControl reports whether r is a Unicode direction control: the marks,
// embeddings, overrides and isolates that can reorder the text around them and
// make a value look like something else. Other format characters, such as the
// joiner in emoji, are left alone.
func isBidiControl(r rune) bool {
	switch {
	case r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	}

	return false
}

// Quote returns s as one word for a shell: as it is when it holds only safe
// characters, else in double quotes, else in single quotes.
func Quote(s string) string {
	if s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !isShellSafe(r) }) {
		return s
	}

	if !strings.ContainsAny(s, "\"$`\\!\n") {
		return `"` + s + `"`
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isShellSafe(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_@%+=:,./-", r)
}

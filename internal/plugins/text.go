package plugins

import (
	"strings"
	"unicode"
)

// SafeText returns text with every control, format, line separator and
// paragraph separator character (Unicode categories Cc, Cf, Zl and Zp) replaced
// by U+FFFD. Plugin names, descriptions and permission details come from
// outside the CLI, so they must not be printed as they are. Control characters
// can drive the terminal, bidirectional controls can reorder what the user
// reads, and zero-width characters can hide text.
func SafeText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return unicode.ReplacementChar
		}

		return r
	}, text)
}

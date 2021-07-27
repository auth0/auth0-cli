package ansi

import (
	"fmt"
	"os"

	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/logrusorgru/aurora"
	"github.com/tidwall/pretty"
)

var darkTerminalStyle = &pretty.Style{
	Key:    [2]string{"\x1B[34m", "\x1B[0m"},
	String: [2]string{"\x1B[30m", "\x1B[0m"},
	Number: [2]string{"\x1B[94m", "\x1B[0m"},
	True:   [2]string{"\x1B[35m", "\x1B[0m"},
	False:  [2]string{"\x1B[35m", "\x1B[0m"},
	Null:   [2]string{"\x1B[31m", "\x1B[0m"},
}

// ForceColors forces the use of colors and other ANSI sequences.
var ForceColors = false

// DisableColors disables all colors and other ANSI sequences.
var DisableColors = false

// EnvironmentOverrideColors overs coloring based on `CLICOLOR` and
// `CLICOLOR_FORCE`. Cf. https://bixense.com/clicolors/
var EnvironmentOverrideColors = true

var color = Color()

// Bold returns bolded text if the writer supports colors
func Bold(text string) string {
	return color.Sprintf(color.Bold(text))
}

// Color returns an aurora.Aurora instance with colors enabled or disabled
// depending on whether the writer supports colors.
func Color() aurora.Aurora {
	return aurora.NewAurora(shouldUseColors())
}

// ColorizeJSON returns a colorized version of the input JSON, if the writer
// supports colors.
func ColorizeJSON(json string, darkStyle bool) string {
	if !shouldUseColors() {
		return json
	}

	style := (*pretty.Style)(nil)
	if darkStyle {
		style = darkTerminalStyle
	}

	return string(pretty.Color([]byte(json), style))
}

// ColorizeStatus returns a colorized number for HTTP status code
func ColorizeStatus(status int) aurora.Value {
	switch {
	case status >= 500:
		return color.Red(status).Bold()
	case status >= 300:
		return color.Yellow(status).Bold()
	default:
		return color.Green(status).Bold()
	}
}

// Faint returns slightly offset color text if the writer supports it
func Faint(text string) string {
	return color.Sprintf(color.Faint(text))
}

// Italic returns italicized text if the writer supports it.
func Italic(text string) string {
	return color.Sprintf(color.Italic(text))
}

// Red returns text colored red
func Red(text string) string {
	return color.Sprintf(color.Red(text))
}

// BrightRed returns text colored bright red
func BrightRed(text string) string {
	return color.Sprintf(color.BrightRed(text))
}

// Green returns text colored green
func Green(text string) string {
	return color.Sprintf(color.Green(text))
}

// Yellow returns text colored yellow
func Yellow(text string) string {
	return color.Sprintf(color.Yellow(text))
}

// BrightYellow returns text colored bright yellow
func BrightYellow(text string) string {
	return color.Sprintf(color.BrightYellow(text))
}

// Blue returns text colored blue
func Blue(text string) string {
	return color.Sprintf(color.Blue(text))
}

// Magenta returns text colored magenta
func Magenta(text string) string {
	return color.Sprintf(color.Magenta(text))
}

// Cyan returns text colored cyan
func Cyan(text string) string {
	return color.Sprintf(color.BrightCyan(text))
}

// Linkify returns an ANSI escape sequence with an hyperlink, if the writer
// supports colors.
func Linkify(text, url string) string {
	if !shouldUseColors() {
		return text
	}

	// See https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
	// for more information about this escape sequence.
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

// StrikeThrough returns struck though text if the writer supports colors
func StrikeThrough(text string) string {
	return color.Sprintf(color.StrikeThrough(text))
}

func shouldUseColors() bool {
	useColors := ForceColors || iostream.IsOutputTerminal()

	if EnvironmentOverrideColors {
		force, ok := os.LookupEnv("CLICOLOR_FORCE")

		switch {
		case ok && force != "0":
			useColors = true
		case ok && force == "0":
			useColors = false
		case os.Getenv("CLICOLOR") == "0":
			useColors = false
		}
	}

	return useColors && !DisableColors
}

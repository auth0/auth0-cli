package ansi

import (
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/tidwall/pretty"

	"github.com/auth0/auth0-cli/internal/iostream"
)

// ForceColors forces the use of colors and other ANSI sequences.
var ForceColors = false

// DisableColors disables all colors and other ANSI sequences.
var DisableColors = false

// EnvironmentOverrideColors overs coloring based on `CLICOLOR` and
// `CLICOLOR_FORCE`. Cf. https://bixense.com/clicolors/
var EnvironmentOverrideColors = true

// Initialize the aurora.Aurora instance. This value needs to be
// set to prevent any runtime errors. Re-initialization of this
// value is done later in the application lifecycle, once the
// color configuration (ex:`--no-color`) is known.
var color = Color()

// Bold returns bolded text if the writer supports colors.
func Bold(text string) string {
	return color.Sprintf(color.Bold(text))
}

// Color returns an aurora.Aurora instance with colors enabled or disabled
// depending on whether the writer supports colors.
func Color() aurora.Aurora {
	return aurora.NewAurora(shouldUseColors())
}

// Initialize re-instantiates the Aurora instance
// This initialization step is necessary because the parsing of the
// --no-color flag is done fairly late in the application cycle.
func Initialize(shouldDisableColors bool) {
	DisableColors = shouldDisableColors
	color = Color()
}

// ColorizeJSON returns a colorized version of the input JSON, if the writer
// supports colors.
func ColorizeJSON(json string) string {
	if !shouldUseColors() {
		return json
	}

	style := (*pretty.Style)(nil)

	return string(pretty.Color([]byte(json), style))
}

// Faint returns slightly offset color text if the writer supports it.
func Faint(text string) string {
	return color.Sprintf(color.Faint(text))
}

// Italic returns italicized text if the writer supports it.
func Italic(text string) string {
	return color.Sprintf(color.Italic(text))
}

// URL formats URL links if the writer supports it
func URL(text string) string {
	return color.Sprintf(color.Underline(text))
}

// Red returns text colored red.
func Red(text string) string {
	return color.Sprintf(color.Red(text))
}

// BrightRed returns text colored bright red.
func BrightRed(text string) string {
	return color.Sprintf(color.BrightRed(text))
}

// Green returns text colored green.
func Green(text string) string {
	return color.Sprintf(color.Green(text))
}

// Yellow returns text colored yellow.
func Yellow(text string) string {
	return color.Sprintf(color.Yellow(text))
}

// BrightYellow returns text colored bright yellow.
func BrightYellow(text string) string {
	return color.Sprintf(color.BrightYellow(text))
}

// Blue returns text colored blue.
func Blue(text string) string {
	return color.Sprintf(color.Blue(text))
}

// Magenta returns text colored magenta.
func Magenta(text string) string {
	return color.Sprintf(color.Magenta(text))
}

// Cyan returns text colored cyan.
func Cyan(text string) string {
	return color.Sprintf(color.BrightCyan(text))
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

package setup

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/text"
)

// CommandOptions are the flags of the setup command that the inputs do not hold.
type CommandOptions struct {
	// Yes adds --yes, which skips the confirmation.
	Yes bool
	// SkipDoctor adds --skip-doctor.
	SkipDoctor bool
}

// Command rebuilds the setup command that does what the inputs ask, as a list of
// parts: the command first, then each flag with its value. Values that need it
// are cleaned of control characters and then quoted for a shell. The dry run flag is never in it, so that the command
// applies the plan, and --yes is in it only when given.
func Command(in Inputs, opts CommandOptions) []string {
	parts := []string{"auth0 poppy setup"}

	add := func(flag, value string) { parts = append(parts, flag+" "+text.Quote(text.Line(value))) }

	if in.CompanyDomain != "" {
		add("--company-domain", in.CompanyDomain)
	}

	for _, api := range in.APIs {
		add("--api", api)
	}

	for _, identifier := range slices.Sorted(maps.Keys(in.APIURLs)) {
		add("--api-url", identifier+"="+in.APIURLs[identifier])
	}

	if in.App != "" {
		add("--app", in.App)
	}

	for _, name := range slices.Sorted(maps.Keys(in.Scopes)) {
		add("--scope", name+"="+in.Scopes[name])
	}

	for _, name := range slices.Sorted(maps.Keys(in.Hints)) {
		hint := in.Hints[name]
		add("--hint", fmt.Sprintf("%s=changes:%s,personal_data:%t", name, hint.Changes, hint.PersonalData))
	}

	if in.Name != "" {
		add("--name", in.Name)
	}

	if in.LoginURI != "" {
		add("--login-uri", in.LoginURI)
	}

	if in.Out != "" && in.Out != config.DefaultFileName {
		add("--out", in.Out)
	}

	parts = append(parts, guestFlags(in)...)

	if in.ForceConfig {
		parts = append(parts, "--force-config")
	}

	if opts.SkipDoctor {
		parts = append(parts, "--skip-doctor")
	}

	if opts.Yes {
		parts = append(parts, "--yes")
	}

	return parts
}

// guestFlags returns the flags that ask for the guest grant. Scopes picked in a
// terminal become --guest-scopes only when the same scopes were picked for every
// API: the flag names one set for all of them, and a rerun must never grant a
// guest more than was chosen. Otherwise the scopes are left out, and a terminal
// asks for them again.
func guestFlags(in Inputs) []string {
	if !in.GuestAPIAccess {
		return nil
	}

	flags := []string{"--guest-api-access"}

	switch {
	case in.GuestAllScopes:
		return append(flags, "--guest-all-scopes")
	case len(in.GuestScopeNames) > 0:
		return append(flags, "--guest-scopes "+text.Quote(text.Line(strings.Join(in.GuestScopeNames, ","))))
	case len(in.GuestScopes) == 0 || len(in.GuestScopes) != len(in.APIs):
		return flags
	}

	var names []string
	for i, identifier := range slices.Sorted(maps.Keys(in.GuestScopes)) {
		picked := slices.Sorted(slices.Values(in.GuestScopes[identifier]))
		if i == 0 {
			names = picked
		} else if !slices.Equal(names, picked) {
			return flags
		}
	}

	if len(names) == 0 {
		return flags
	}

	return append(flags, "--guest-scopes "+text.Quote(strings.Join(names, ",")))
}

// FormatCommand joins the parts of a command after a label. When width is more
// than zero it breaks the command before a part that would pass the width, ending
// the line with a backslash and indenting the next one under the command.
func FormatCommand(label string, parts []string, width int) string {
	if width <= 0 {
		return label + strings.Join(parts, " ")
	}

	var b strings.Builder

	indent := strings.Repeat(" ", utf8.RuneCountInString(label)+2)
	line := label + parts[0]

	for _, part := range parts[1:] {
		if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(part)+2 > width {
			b.WriteString(line + " \\\n")
			line = indent + part

			continue
		}

		line += " " + part
	}

	b.WriteString(line)

	return b.String()
}

package setup

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/text"
)

// CommandOptions are the flags of the setup command that the inputs do not hold.
type CommandOptions struct {
	// Tenant is the domain of the tenant the command is for. When it is set,
	// --tenant comes first among the flags, so that pasting the command cannot
	// change another tenant than the one planned.
	Tenant string
	// Yes adds --force, which skips the confirmation.
	Yes bool
	// Config builds the auth0 poppy config command, which has no --scope,
	// --login-uri or --force.
	Config bool
}

// Command rebuilds the setup command that does what the inputs ask, as a list of
// parts: the command first, then each flag with its value. Values that need it
// are cleaned of control characters and then quoted for a shell. The dry run flag
// is never in it, so that the command applies the plan, and --force is in it only
// when given.
func Command(in Inputs, opts CommandOptions) []string {
	parts := []string{"auth0 poppy setup"}
	if opts.Config {
		parts = []string{"auth0 poppy config"}
	}

	add := func(flag, value string) { parts = append(parts, flag+" "+text.Quote(text.Line(value))) }

	if opts.Tenant != "" {
		add("--tenant", opts.Tenant)
	}

	if in.CompanyDomain != "" {
		add("--company-domain", in.CompanyDomain)
	}

	if in.NoAPI {
		parts = append(parts, "--no-api")
	}

	for _, api := range in.APIs {
		add("--api", api)
	}

	for _, identifier := range slices.Sorted(maps.Keys(in.APIURLs)) {
		add("--api-url", identifier+"="+in.APIURLs[identifier])
	}

	if in.NoWeb {
		parts = append(parts, "--no-web")
	}

	if in.App != "" {
		add("--app", in.App)
	}

	for _, name := range slices.Sorted(maps.Keys(in.Scopes)) {
		if !opts.Config {
			add("--scope", name+"="+in.Scopes[name])
		}
	}

	for _, name := range slices.Sorted(maps.Keys(in.Hints)) {
		hint := in.Hints[name]
		add("--hint", fmt.Sprintf("%s=changes:%s,personal_data:%t", name, hint.Changes, hint.PersonalData))
	}

	if in.Name != "" {
		add("--company-name", in.Name)
	}

	if in.LoginURI != "" && !opts.Config {
		add("--login-uri", in.LoginURI)
	}

	if in.Out != "" && in.Out != config.DefaultFileName {
		add("--out", in.Out)
	}

	if in.WebUnscoped != nil {
		parts = append(parts, fmt.Sprintf("--web-unscoped=%t", *in.WebUnscoped))
	}

	if in.ForceConfig {
		parts = append(parts, "--overwrite-config")
	}

	if opts.Yes && !opts.Config {
		parts = append(parts, "--force")
	}

	return parts
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
			b.WriteString(line)
			b.WriteString(" \\\n")
			line = indent + part

			continue
		}

		line += " " + part
	}

	b.WriteString(line)

	return b.String()
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func init() {
	cobra.AddTemplateFunc("WrappedInheritedFlagUsages", WrappedInheritedFlagUsages)
	cobra.AddTemplateFunc("WrappedLocalFlagUsages", WrappedLocalFlagUsages)
	cobra.AddTemplateFunc("WrappedRequestParamsFlagUsages", WrappedRequestParamsFlagUsages)
	cobra.AddTemplateFunc("WrappedNonRequestParamsFlagUsages", WrappedNonRequestParamsFlagUsages)
	cobra.AddTemplateFunc("WrappedAliases", WrappedAliases)
}

// WrappedInheritedFlagUsages returns a string containing the usage information
// for all flags which were inherited from parent commands, wrapped to the
// terminal's width.
func WrappedInheritedFlagUsages(cmd *cobra.Command) string {
	return cmd.InheritedFlags().FlagUsagesWrapped(getTerminalWidth())
}

// WrappedLocalFlagUsages returns a string containing the usage information
// for all flags specifically set in the current command, wrapped to the
// terminal's width.
func WrappedLocalFlagUsages(cmd *cobra.Command) string {
	return cmd.LocalFlags().FlagUsagesWrapped(getTerminalWidth())
}

// WrappedRequestParamsFlagUsages returns a string containing the usage
// information for all request parameters flags, i.e. flags used in operation
// commands to set values for request parameters. The string is wrapped to the
// terminal's width.
func WrappedRequestParamsFlagUsages(cmd *cobra.Command) string {
	var sb strings.Builder

	// We're cheating a little bit in thie method: we're not actually wrapping
	// anything, just printing out the flag names and assuming that no name
	// will be long enough to go over the terminal's width.
	// We do this instead of using pflag's `FlagUsagesWrapped` function because
	// we don't want to print the types (all request parameters flags are
	// defined as strings in the CLI, but it would be confusing to print that
	// out as a lot of them are not strings in the API).
	// If/when we do add help strings for request parameters flags, we'll have
	// to do actual wrapping.
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if _, ok := flag.Annotations["request"]; ok {
			sb.WriteString(fmt.Sprintf("      --%s\n", flag.Name))
		}
	})

	return sb.String()
}

// WrappedNonRequestParamsFlagUsages returns a string containing the usage
// information for all non-request parameters flags. The string is wrapped to
// the terminal's width.
func WrappedNonRequestParamsFlagUsages(cmd *cobra.Command) string {
	nonRequestParamsFlags := pflag.NewFlagSet("request", pflag.ExitOnError)

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if _, ok := flag.Annotations["request"]; !ok {
			nonRequestParamsFlags.AddFlag(flag)
		}
	})

	return nonRequestParamsFlags.FlagUsagesWrapped(getTerminalWidth())
}

// WrappedAliases returns a formatted string containing the command aliases if defined, otherwise an empty string.
func WrappedAliases(cmd *cobra.Command) string {
	if len(cmd.Aliases) > 0 {
		return ansi.Faint(ansi.Italic(fmt.Sprintf("[%s]", strings.Join(cmd.Aliases, ", "))))
	}
	return ""
}

func getLogin(cli *cli) string {
	if !cli.isLoggedIn() {
		return ansi.Italic(`
Before using the CLI, you'll need to login:

  $ auth0 login
`)
	}

	return ""
}

func namespaceUsageTemplate() string {
	return fmt.Sprintf(`%s{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} <resource> <operation> [parameters...] [flags]{{end}}{{if gt (len .Aliases) 0}}

%s
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

%s
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%s{{range .Commands}}{{if (.IsAvailableCommand)}}
  {{rpad .Name .NamePadding }} {{.Short}} {{WrappedAliases .}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s
{{WrappedLocalFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s
{{WrappedInheritedFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

%s{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`,
		ansi.Bold("Usage:"),
		ansi.Bold("Aliases:"),
		ansi.Bold("Examples:"),
		ansi.Bold("Available Resources:"),
		ansi.Bold("Flags:"),
		ansi.Bold("Global Flags:"),
		ansi.Bold("Additional help topics:"),
	)
}

func resourceUsageTemplate() string {
	return fmt.Sprintf(`%s{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} <operation> [parameters...] [flags]{{end}}{{if gt (len .Aliases) 0}}

%s
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

%s
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%s{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}} {{WrappedAliases .}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s
{{WrappedLocalFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s
{{WrappedInheritedFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

%s{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`,
		ansi.Bold("Usage:"),
		ansi.Bold("Aliases:"),
		ansi.Bold("Examples:"),
		ansi.Bold("Available Operations:"),
		ansi.Bold("Flags:"),
		ansi.Bold("Global Flags:"),
		ansi.Bold("Additional help topics:"),
	)
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(0)
	if err != nil {
		width = 80
	}

	return width
}

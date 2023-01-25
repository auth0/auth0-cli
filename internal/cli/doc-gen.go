package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const (
	docsPath = "./docs/"

	associatedCommandsFragment = `{{range .AssociatedCommands}}- [{{.CommandPath}}]({{ link .CommandPath }}) - {{.Short}}
{{end}}`

	parentPageTemplate = `---
layout: default
has_toc: false
{{- if eq .HasRunnableChildren true }}
has_children: true
{{- end}}
{{- if ne .ParentCommandPath "" }}
parent: {{.ParentCommandPath}}
{{- end}}
---
# {{.Name}}

{{.Description}}

## Commands

%s
`

	homepageTemplate = `---
layout: home
---

Build, manage and test your [Auth0](https://auth0.com/) integrations from the command line.

## Installation

### macOS

Install via [Homebrew](https://brew.sh/):

{{ wrapWithBackticks "brew tap auth0/auth0-cli && brew install auth0" }}

### Windows

Install via [Scoop](https://scoop.sh/):

{{ wrapWithBackticks "scoop bucket add auth0 https://github.com/auth0/scoop-auth0-cli.git && scoop install auth0" }}

### Linux

Install via [cURL](https://curl.se/):

{{ wrapWithBackticks "curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/main/install.sh | sh -s -- -b ." }}

### Go

Install via [Go](https://go.dev/):

{{ wrapWithBackticks "go install github.com/auth0/auth0-cli/cmd/auth0@latest" }}

### Manual

1. Download the appropriate binary for your environment from the [latest release](https://github.com/auth0/auth0-cli/releases/latest/)
2. Extract the archive
3. Run ` + "`./auth0`" + `

Autocompletion instructions for supported platforms available by running ` + "`auth0 completion -h`" + `

## Authenticating to Your Tenant

Authenticating to your Auth0 tenant is required for most functions of the CLI. It can be initiated by running:

{{ wrapWithBackticks "auth0 login" }}

There are two ways to authenticate:

- **As a user** - Recommended when invoking on a personal machine or other interactive environment. Facilitated by [device authorization](https://auth0.com/docs/get-started/authentication-and-authorization-flow/device-authorization-flow) flow.
- **As a machine** - Recommended when running on a server or non-interactive environments (ex: CI). Facilitated by [client credentials](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow) flow. Flags available for bypassing interactive shell.

> ⚠️ Authenticating as a user is not supported for **private cloud** tenants. Instead, those users should authenticate with client credentials.

## Available Commands

%s
`

	commandPageTemplate = `---
layout: default
{{- if ne .ParentCommandPath "auth0" }}
parent: {{.ParentCommandPath}}
{{- end}}
has_toc: false
---
# {{.Name}}

{{.Description}}

## Usage
{{ wrapWithBackticks .UseLine }}

## Examples

{{ wrapWithBackticks .Examples }}

{{if .HasFlags}}
## Flags

{{ wrapWithBackticks .Flags }}{{end}}

{{if .HasInheritedFlags}}
## Inherited Flags

{{ wrapWithBackticks .InheritedFlags }}{{end}}

{{ if .AssociatedCommands }}
## Related Commands

%s
{{ end }}
`
)

type page struct {
	Name                string
	HasFlags            bool
	Flags               string
	HasInheritedFlags   bool
	InheritedFlags      string
	Description         string
	AssociatedCommands  []*cobra.Command
	Examples            string
	UseLine             string
	CommandPath         string
	ParentCommandPath   string
	HasRunnableChildren bool
}

// GenerateDocs will generate the documentation
// for all the commands under the ./docs folder.
func GenerateDocs() error {
	baseCmd := &cobra.Command{
		Use:   "auth0",
		Short: rootShort,
	}
	baseCmd.SetUsageTemplate(namespaceUsageTemplate())

	cli := &cli{}
	addPersistentFlags(baseCmd, cli)
	addSubCommands(baseCmd, cli)

	return GenMarkdownTree(baseCmd, docsPath)
}

// GenMarkdownTree is the same as GenMarkdownTree, but with linkHandler.
func GenMarkdownTree(cmd *cobra.Command, dir string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTree(c, dir); err != nil {
			return err
		}
	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	isHomepage := cmd.CommandPath() == "auth0"
	if isHomepage {
		return GenerateHomepage(cmd, f)
	}

	isParentPage := !cmd.Runnable()
	if isParentPage {
		return GenerateParentPage(cmd, f)
	}

	return GenerateCommandPage(cmd, f)
}

// GenerateHomepage creates custom markdown for the homepage.
func GenerateHomepage(cmd *cobra.Command, w io.Writer) error {
	templateBody := fmt.Sprintf(homepageTemplate, associatedCommandsFragment)

	pageData := page{
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Commands(),
	}

	return GeneratePage(w, "homepageTemplate", templateBody, pageData)
}

// GenerateParentPage creates custom markdown for the parent command pages.
func GenerateParentPage(cmd *cobra.Command, w io.Writer) error {
	templateBody := fmt.Sprintf(parentPageTemplate, associatedCommandsFragment)

	hasRunnableChildren := false
	for _, c := range cmd.Commands() {
		if c.Runnable() {
			hasRunnableChildren = true
		}
	}

	parentCommand := ""
	if cmd.Parent().Runnable() {
		parentCommand = cmd.Parent().CommandPath()
	}

	pageData := page{
		Name:                cmd.CommandPath(),
		Description:         cmd.Long,
		CommandPath:         cmd.CommandPath(),
		AssociatedCommands:  cmd.Commands(),
		ParentCommandPath:   parentCommand,
		HasRunnableChildren: hasRunnableChildren,
	}

	return GeneratePage(w, "parentPageTemplate", templateBody, pageData)
}

// GenerateCommandPage creates custom markdown for the individual command pages.
func GenerateCommandPage(cmd *cobra.Command, w io.Writer) error {
	templateBody := fmt.Sprintf(commandPageTemplate, associatedCommandsFragment)

	parentCommand := cmd.Parent()

	associatedCommands := parentCommand.Commands()
	if parentCommand.Name() == "auth0" {
		associatedCommands = nil
	}

	pageData := page{
		Name:               cmd.CommandPath(),
		Description:        cmd.Long,
		HasFlags:           cmd.HasLocalFlags(),
		Flags:              cmd.NonInheritedFlags().FlagUsages(),
		HasInheritedFlags:  cmd.HasInheritedFlags(),
		InheritedFlags:     cmd.InheritedFlags().FlagUsages(),
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: associatedCommands,
		Examples:           cmd.Example,
		UseLine:            cmd.UseLine(),
		ParentCommandPath:  parentCommand.CommandPath(),
	}

	return GeneratePage(w, "commandPageTemplate", templateBody, pageData)
}

func GeneratePage(w io.Writer, name, body string, data interface{}) error {
	t, err := template.New(name).
		Funcs(template.FuncMap{
			"link": func(commandPath string) string {
				return strings.ReplaceAll(commandPath, " ", "_") + ".md"
			},
			"wrapWithBackticks": func(body string) string {
				body = strings.TrimSuffix(body, "\n")
				return fmt.Sprintf("```\n%s\n```", body)
			},
		}).
		Parse(body)
	if err != nil {
		return err
	}

	var page bytes.Buffer
	if err = t.Execute(&page, data); err != nil {
		return err
	}

	_, err = page.WriteTo(w)
	return err
}

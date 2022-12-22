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
---
# {{.Name}}

{{.Description}}

## Commands

%s
`

	homepageTemplate = `---
layout: home
---

Build, manage and test your [Auth0](http://auth0.com/) integrations from the command line.

## Installation

Installation instructions available in [project README](https://github.com/auth0/auth0-cli#installation).

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
---
# {{.Name}}

{{.Description}}

{{ wrapWithBackticks .UseLine }}

{{if .HasFlags}}
## Flags

{{ wrapWithBackticks .Flags }}{{end}}

{{if .HasInheritedFlags}}
## InheritedFlags

{{ wrapWithBackticks .InheritedFlags }}{{end}}

## Examples

{{ wrapWithBackticks .Examples }}

## Related Commands

%s
`
)

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

	pageData := struct {
		CommandPath        string
		AssociatedCommands []*cobra.Command
	}{
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Commands(),
	}

	return GeneratePage(w, "homepageTemplate", templateBody, pageData)
}

// GenerateParentPage creates custom markdown for the parent command pages.
func GenerateParentPage(cmd *cobra.Command, w io.Writer) error {
	templateBody := fmt.Sprintf(parentPageTemplate, associatedCommandsFragment)

	pageData := struct {
		Name               string
		Description        string
		CommandPath        string
		AssociatedCommands []*cobra.Command
	}{
		Name:               cmd.CommandPath(),
		Description:        cmd.Long,
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Commands(),
	}

	return GeneratePage(w, "parentPageTemplate", templateBody, pageData)
}

// GenerateCommandPage creates custom markdown for the individual command pages.
func GenerateCommandPage(cmd *cobra.Command, w io.Writer) error {
	templateBody := fmt.Sprintf(commandPageTemplate, associatedCommandsFragment)

	pageData := struct {
		Name               string
		HasFlags           bool
		Flags              string
		HasInheritedFlags  bool
		InheritedFlags     string
		Description        string
		CommandPath        string
		AssociatedCommands []*cobra.Command
		Examples           string
		UseLine            string
	}{
		Name:               cmd.CommandPath(),
		Description:        cmd.Long,
		HasFlags:           cmd.HasLocalFlags(),
		Flags:              cmd.NonInheritedFlags().FlagUsages(),
		HasInheritedFlags:  cmd.HasInheritedFlags(),
		InheritedFlags:     cmd.InheritedFlags().FlagUsages(),
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Parent().Commands(),
		Examples:           cmd.Example,
		UseLine:            cmd.UseLine(),
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

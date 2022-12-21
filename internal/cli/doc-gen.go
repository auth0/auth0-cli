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
	docsPath                   = "./docs/"
	associatedCommandsFragment = `{{range .AssociatedCommands}}- [{{.CommandPath}}](auth0_{{.Name}}.md) - {{.Short}}
{{end}}`
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

	return GenMarkdownTreeCustom(baseCmd, docsPath)
}

// GenMarkdownTreeCustom is the the same as GenMarkdownTree, but with linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTreeCustom(c, dir); err != nil {
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

	homepageTemplate :=
		`---
layout: home
---

Build, manage and test your [Auth0](http://auth0.com/) integrations from the command line.

## Installation

Installation instructions available in [project README](https://github.com/auth0/auth0-cli#installation).

## Authenticating to Your Tenant

Authenticating to your Auth0 tenant is required for most functions of the CLI. It can be initiated by running:

{{.LoginCommand}}

There are two ways to authenticate:

- **As a user** - Recommended when invoking on a personal machine or other interactive environment. Facilitated by [device authorization](https://auth0.com/docs/get-started/authentication-and-authorization-flow/device-authorization-flow) flow.
- **As a machine** - Recommended when running on a server or non-interactive environments (ex: CI). Facilitated by [client credentials](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow) flow. Flags available for bypassing interactive shell.

> **Warning**
> Authenticating as a user is not supported for **private cloud** tenants. Instead, those users should authenticate with client credentials.

## Available Commands

%s
`
	var tpl bytes.Buffer
	t := template.Must(template.New("homepageTemplate").Parse(fmt.Sprintf(homepageTemplate, associatedCommandsFragment)))

	if err := t.Execute(&tpl, struct {
		CommandPath        string
		LoginCommand       string
		AssociatedCommands []*cobra.Command
	}{
		LoginCommand:       wrapWithBackticks("auth0 login"),
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Commands(),
	}); err != nil {
		return err
	}

	_, err := tpl.WriteTo(w)
	return err
}

// GenerateParentPage creates custom markdown for the parent command pages.
func GenerateParentPage(cmd *cobra.Command, w io.Writer) error {

	parentPageTemplate :=
		`---
layout: default
---
# {{.Name}}

{{.Description}}

## Commands

%s
`
	var tpl bytes.Buffer
	t := template.Must(template.New("parentPageTemplate").Parse(fmt.Sprintf(parentPageTemplate, associatedCommandsFragment)))

	err := t.Execute(&tpl, struct {
		Name               string
		Description        string
		CommandPath        string
		AssociatedCommands []*cobra.Command
	}{
		Name:               fmt.Sprintf("`%s`", cmd.CommandPath()),
		Description:        cmd.Long,
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: cmd.Commands(),
	})
	if err != nil {
		return err
	}

	_, err = tpl.WriteTo(w)
	return err
}

// GenerateCommandPage creates custom markdown for the individual command pages.
func GenerateCommandPage(cmd *cobra.Command, w io.Writer) error {

	commandPageTemplate :=
		`---
layout: default
---
# {{.Name}}

{{.Description}}

{{.UseLine}}

{{if .HasFlags}}
## Flags

{{.Flags}}{{end}}

{{if .HasInheritedFlags}}
## InheritedFlags

{{.InheritedFlags}}{{end}}

## Examples

{{.Examples}}

## Related Commands

%s
`
	var tpl bytes.Buffer
	t := template.Must(template.New("commandPageTemplate").Parse(fmt.Sprintf(commandPageTemplate, associatedCommandsFragment)))

	relatedCommands := cmd.Parent().Commands()

	err := t.Execute(&tpl, struct {
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
		Name:               fmt.Sprintf("`%s`", cmd.CommandPath()),
		Description:        cmd.Long,
		HasFlags:           cmd.HasLocalFlags(),
		Flags:              wrapWithBackticks(cmd.NonInheritedFlags().FlagUsages()),
		HasInheritedFlags:  cmd.HasInheritedFlags(),
		InheritedFlags:     wrapWithBackticks(cmd.InheritedFlags().FlagUsages()),
		CommandPath:        cmd.CommandPath(),
		AssociatedCommands: relatedCommands,
		Examples:           wrapWithBackticks(cmd.Example),
		UseLine:            wrapWithBackticks(cmd.UseLine()),
	})
	if err != nil {
		return err
	}

	_, err = tpl.WriteTo(w)
	return err
}

func wrapWithBackticks(body string) string {
	return fmt.Sprintf("```\n%s\n```", body)
}

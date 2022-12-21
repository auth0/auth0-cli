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
)

// GenerateDocs will generate the documentation
// for all the commands under the ./docs folder.
func GenerateDocs() error {
	baseCmd := &cobra.Command{
		Use:               "auth0",
		Short:             rootShort,
		DisableAutoGenTag: false,
	}
	baseCmd.SetUsageTemplate(namespaceUsageTemplate())

	cli := &cli{}
	addPersistentFlags(baseCmd, cli)
	addSubCommands(baseCmd, cli)

	return GenMarkdownTreeCustom(baseCmd, docsPath, handleLinks)
}

// GenMarkdownTreeCustom is the the same as GenMarkdownTree, but with linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTreeCustom(c, dir, linkHandler); err != nil {
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
		return GenerateHomepage(cmd, f, linkHandler)
	}

	isParentPage := !cmd.Runnable()
	if isParentPage {
		return GenerateParentPage(cmd, f, linkHandler)
	}

	return GenerateCommandPage(cmd, f, linkHandler)
}

// GenerateHomepage creates custom markdown output.
func GenerateHomepage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	homepageTemplate :=
		`---
layout: home
---
## Authenticating With Your Tenant

This is how you authenticate with your tenant...

## Installation

Installation instructions available on [project README](https://github.com/auth0/auth0-cli#installation)

## Available Commands

{{range .AvailableCommands}}* [{{.CommandPath}}](auth0_{{.Name}}.md) - {{.Short}}
{{end}}
`
	var tpl bytes.Buffer
	t := template.Must(template.New("homepageTemplate").Parse(homepageTemplate))

	if err := t.Execute(&tpl, struct {
		CommandPath       string
		AvailableCommands []*cobra.Command
	}{
		CommandPath:       cmd.CommandPath(),
		AvailableCommands: cmd.Commands(),
	}); err != nil {
		return err
	}

	_, err := tpl.WriteTo(w)
	return err
}

// GenerateParentPage creates custom markdown output.
func GenerateParentPage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	parentPageTemplate :=
		`---
layout: default
---
# {{.Name}}

{{.Description}}

## Commands

{{range .AvailableCommands}}* [{{.CommandPath}}](auth0_{{.Name}}.md) - {{.Short}}
{{end}}
`
	var tpl bytes.Buffer
	t := template.Must(template.New("parentPageTemplate").Parse(parentPageTemplate))

	err := t.Execute(&tpl, struct {
		Name              string
		Description       string
		CommandPath       string
		AvailableCommands []*cobra.Command
	}{
		Name:              cmd.Name(),
		Description:       cmd.Long,
		CommandPath:       cmd.CommandPath(),
		AvailableCommands: cmd.Commands(),
	})
	if err != nil {
		return err
	}

	_, err = tpl.WriteTo(w)
	return err
}

// GenerateCommandPage creates custom markdown output.
func GenerateCommandPage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	commandPageTemplate :=
		`---
layout: default
---
# {{.Name}}

{{.Description}}

{{.UseLine}}

## Flags

{{.Flags}}

## InheritedFlags

{{.InheritedFlags}}

## Examples

{{.Examples}}

## Related Commands

{{range .RelatedCommands}}* [{{.CommandPath}}](auth0_{{.Name}}.md) - {{.Short}}
{{end}}
`
	var tpl bytes.Buffer
	t := template.Must(template.New("commandPageTemplate").Parse(commandPageTemplate))

	err := t.Execute(&tpl, struct {
		Name            string
		Flags           string
		InheritedFlags  string
		Description     string
		CommandPath     string
		RelatedCommands []*cobra.Command
		Examples        string
		UseLine         string
	}{
		Name:            fmt.Sprintf("`%s`", cmd.CommandPath()),
		Description:     cmd.Long,
		Flags:           wrapWithBackticks(cmd.NonInheritedFlags().FlagUsages()),
		InheritedFlags:  wrapWithBackticks(cmd.InheritedFlags().FlagUsages()),
		CommandPath:     cmd.CommandPath(),
		RelatedCommands: cmd.Commands(),
		Examples:        wrapWithBackticks(cmd.Example),
		UseLine:         wrapWithBackticks(cmd.UseLine()),
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

func handleLinks(fileName string) string {
	if isIndexFile(fileName) {
		return "/auth0-cli/"
	}

	return fileName
}

func isIndexFile(fileName string) bool {
	return strings.HasSuffix(fileName, "auth0.md")
}

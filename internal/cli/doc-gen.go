package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	docsPath   = "./docs/"
	homeLayout = `---
layout: home
---
`
	defaultLayout = `---
layout: default
---
`
)

// GenerateDocs will generate the documentation
// for all the commands under the ./docs folder.
func GenerateDocs() error {

	fmt.Println("GENERATING")
	baseCmd := &cobra.Command{
		Use:               "auth0",
		Short:             rootShort,
		DisableAutoGenTag: false,
	}
	baseCmd.SetUsageTemplate(namespaceUsageTemplate())

	cli := &cli{}
	addPersistentFlags(baseCmd, cli)
	addSubCommands(baseCmd, cli)

	return GenMarkdownTreeCustom(baseCmd, docsPath, prependToFiles, handleLinks)
}

// GenMarkdownTreeCustom is the the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, filePrepender, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
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

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}

	isHomepage := cmd.CommandPath() == "auth0"

	if isHomepage {
		if err := GenerateHomepage(cmd, f, linkHandler); err != nil {
			return err
		}
		return nil
	}

	isParentPage := !cmd.Runnable()

	if isParentPage {
		if err := GenerateParentPage(cmd, f, linkHandler); err != nil {
			return err
		}
		return nil
	}

	if err := GenerateCommandPage(cmd, f, linkHandler); err != nil {
		return err
	}

	return nil
}

// GenerateHomepage creates custom markdown output.
func GenerateHomepage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	homepageContent := `
## Authenticating With Your Tenant

foo bar this is how you authenticate

## Installation

Installation instructions available on [project README](https://github.com/auth0/auth0-cli#installation)
`

	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	buf.WriteString(homepageContent)

	if hasRelatedCommands(cmd) {
		buf.WriteString("## Available Commands\n\n")

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			link := cname + ".md"
			link = strings.ReplaceAll(link, " ", "_")
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", cname, linkHandler(link), child.Short))
		}
		buf.WriteString("\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

// GenerateParentPage creates custom markdown output.
func GenerateParentPage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	buf.WriteString("# " + name + "\n\n")
	buf.WriteString(cmd.Long + "\n\n")

	if hasRelatedCommands(cmd) {
		buf.WriteString("## Commands\n\n")

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			link := cname + ".md"
			link = strings.ReplaceAll(link, " ", "_")
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", cname, linkHandler(link), child.Short))
		}
		buf.WriteString("\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

// GenerateCommandPage creates custom markdown output.
func GenerateCommandPage(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {

	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	buf.WriteString("# " + name + "\n\n")
	buf.WriteString(cmd.Long + "\n\n")

	if cmd.Runnable() {
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.UseLine()))
	}

	if len(cmd.Example) > 0 {
		buf.WriteString("## Examples\n\n")
		buf.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.Example))
	}

	if err := printOptions(buf, cmd, name); err != nil {
		return err
	}
	if hasRelatedCommands(cmd) {
		buf.WriteString("## Related Commands\n\n")

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			cname := name + " " + child.Name()
			link := cname + ".md"
			link = strings.ReplaceAll(link, " ", "_")
			buf.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", cname, linkHandler(link), child.Short))
		}
		buf.WriteString("\n")
	}
	_, err := buf.WriteTo(w)
	return err
}

func printOptions(buf *bytes.Buffer, cmd *cobra.Command, name string) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(buf)
	if flags.HasAvailableFlags() {
		buf.WriteString("## Flags\n\n```\n")
		flags.PrintDefaults()
		buf.WriteString("```\n\n")
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(buf)
	if parentFlags.HasAvailableFlags() {
		buf.WriteString("## Inherited flags\n\n```\n")
		parentFlags.PrintDefaults()
		buf.WriteString("```\n\n")
	}
	return nil
}

type byName []*cobra.Command

func (s byName) Len() int           { return len(s) }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }

// Test to see if we have a reason to print See Also information in docs
// Basically this is a test for a parent command or a subcommand which is
// both not deprecated and not the autogenerated help command.
func hasRelatedCommands(cmd *cobra.Command) bool {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		return true
	}
	return false
}

func prependToFiles(fileName string) string {
	if isIndexFile(fileName) {
		return homeLayout
	}

	return defaultLayout
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

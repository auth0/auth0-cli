package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
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
	baseCmd := &cobra.Command{
		Use:               "auth0",
		Short:             rootShort,
		DisableAutoGenTag: true,
	}
	baseCmd.SetUsageTemplate(namespaceUsageTemplate())

	cli := &cli{}
	addPersistentFlags(baseCmd, cli)
	addSubCommands(baseCmd, cli)

	return doc.GenMarkdownTreeCustom(baseCmd, docsPath, prependToFiles, handleLinks)
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

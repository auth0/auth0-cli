package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func BuildDoc(path string) error {
	cli := &cli{}

	rootCmd := &cobra.Command{
		Use:               "auth0",
		Short:             rootShort,
		DisableAutoGenTag: true,
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	addPersistentFlags(rootCmd, cli)
	addSubcommands(rootCmd, cli)

	err := doc.GenMarkdownTreeCustom(rootCmd,
		path,
		func(fileName string) string {
			// prepend to the generated markdown
			if strings.HasSuffix(fileName, "auth0.md") {
				return `---
layout: home
---
`
			}

			return `---
layout: default
---
`
		},
		func(fileName string) string {
			// return same value, we're not changing the internal link
			if strings.HasSuffix(fileName, "auth0.md") {
				return "/auth0-cli/"
			}

			return fileName
		})

	if err != nil {
		return err
	}

	return nil
}

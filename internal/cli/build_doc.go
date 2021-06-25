package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func BuildDoc(path string) error {
	cli := &cli{}

	rootCmd := &cobra.Command{
		Use: "index",
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	addPersistentFlags(rootCmd, cli)
	addSubcommands(rootCmd, cli)

	err := doc.GenMarkdownTreeCustom(rootCmd,
		path,
		func(fileName string) string {
			// prepend to the generated markdown
			if strings.HasSuffix(fileName, "index.md") {
				return "---\nlayout: home\n----"
			}

			return "---\nlayout: default\n----"

		},
		func(s string) string {
			// return same value, we're not changing the internal link
			return s
		})

	if err != nil {
		return err
	}

	return nil
}

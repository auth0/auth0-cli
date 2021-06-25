package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func BuildDoc(path string) error {
	cli := &cli{}

	rootCmd := &cobra.Command{
		Use:   "auth0",
		Short: rootShort,
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	addPersistentFlags(rootCmd, cli)
	addSubcommands(rootCmd, cli)

	err := doc.GenMarkdownTree(rootCmd, path)
	if err != nil {
		return err
	}

	return nil
}

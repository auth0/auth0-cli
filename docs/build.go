package main

import (
	"log"

	cli "github.com/auth0/auth0-cli/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	cmd := cli.BuildRootCmd()
	err := doc.GenMarkdownTree(cmd, "./commands")
	if err != nil {
		log.Fatal(err)
	}
}

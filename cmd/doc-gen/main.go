package main

import (
	"log"

	"github.com/auth0/auth0-cli/internal/cli"
)

func main() {
	if err := cli.GenerateDocs(); err != nil {
		log.Fatal(err)
	}
}

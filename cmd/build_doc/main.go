package main

import (
	"github.com/auth0/auth0-cli/internal/cli"
)

func main() {
	err := cli.BuildDoc()
	if err != nil {
		panic(err)
	}
}

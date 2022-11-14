package main

import (
	"github.com/joeshaw/envdecode"

	"github.com/auth0/auth0-cli/internal/cli"
)

func main() {
	var cfg struct {
		Path string `env:"AUTH0_CLI_DOCS_PATH,default=./docs/"`
	}
	if err := envdecode.StrictDecode(&cfg); err != nil {
		panic(err)
	}

	err := cli.BuildDoc(cfg.Path)
	if err != nil {
		panic(err)
	}
}

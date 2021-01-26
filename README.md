# auth0-cli

## What?

The goal is to build a fully fleshed out product, similar to the Heroku CLI,
Stripe CLI, etc.

## Why now?

- It would also allow MVP products to be shipped faster.
- For actions, delivering a CLI experience would be far ideal than having
  developers write code in the browser.

## Installation
### macOS
1. Download the binaries from the latest release: https://github.com/auth0/auth0-cli/releases/latest/
1. Extract
1. Move to `auth0` to `/usr/local/bin/auth0`, e.g.: `mv ~/Desktop/auth0 /usr/local/bin`
1. Setup CLI commands completion for your terminal:
	-  (**bash**) `auth0 completion bash > /usr/local/etc/bash_completion.d/auth0`
	-  (**zsh**)  `auth0 completion zsh > "${fpath[1]}/_auth0"`
	- (**fish**)  `auth0 completion fish | source`

> see full completion options: `auth0 completion -h`

## Dev Setup instructions

1. [Setup go](https://golang.org/doc/install)
2. Clone this repo: `git clone git@github.com:auth0/auth0-cli`
3. `make test` - ensure everything works correctly. Should see things pass.

## Build and run on native platform

From the top-level directory:
```
$ make build
$ ./auth0 --help
```

## Adding a new command

This part is not fully fleshed out yet, but here are the steps:

1. Create a command (example: https://github.com/auth0/auth0-cli/blob/main/internal/cli/login.go)
2. Add the command constructor to the root command: (e.g. somewhere here: https://github.com/auth0/auth0-cli/blob/main/internal/cli/root.go)

Test it out by doing:

```
go run ./cmd/auth0 <your command>
```

## Adding a new go dependency

If you have to add another go dependency, you can follow the steps:

1. `go get -u github.com/some/path/to/lib`
2. Import the library you need in the relevant file. (This step is necessary, so
   the next steps informs `go mod` that this dependency is actually used).
3. go mod tidy
4. go mod vendor

We use vendoring, so the last step is required.

## References

https://auth0team.atlassian.net/wiki/spaces/eco/pages/1050510482/actions%3A+CLI+sketch

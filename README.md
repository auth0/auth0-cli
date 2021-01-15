# auth0-cli

## What?

The goal is to build a fully fleshed out product, similar to the Heroku CLI,
Stripe CLI, etc.

## Why now?

- It would also allow MVP products to be shipped faster.
- For actions, delivering a CLI experience would be far ideal than having
  developers write code in the browser.

## Setup instructions

1. [Setup go](https://golang.org/doc/install)
2. Clone this repo (git clone git@github.com:auth0/auth0-cli
3. `make test` - ensure everything works correctly. Should see things pass.

## Adding a new go dependency

If you have to add another go dependency, you can follow the steps:

1. `go get -u github.com/some/path/to/lib`
2. Import the library you need in the relevant file. (This step is necessary so
   the next steps informs `go mod` that this dependency is actually used).
3. go mod tidy
4. go mod vendor

We use vendoring so the last step is required.

## References

https://auth0team.atlassian.net/wiki/spaces/eco/pages/1050510482/actions%3A+CLI+sketch

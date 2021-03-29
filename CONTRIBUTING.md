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

## Releasing a new version 

> This is only possible if you're a repository maintainer.

The release is driven by a Github **workflow** triggered when a new **tag** is **created**. The workflow will run the checks and trigger _Goreleaser_ to:
- create the Release with the proper description (changelog)
- upload the binaries for the different architectures
- update https://github.com/auth0/homebrew-auth0-cli with the latest binary reference

To release a new version:

1. pull the latest changes: 
   - `$ git checkout main`
   - `$ git pull origin main`
2. check the latest tag: 
   - `$ git fetch`
   -  `$ git tags`
3. create the **new** tag for the new release. For example, if the latest tag is `v0.1.1`, you should create `v.0.1.1`:
   - `$ git tag v0.1.2`
4. push the new tag: 
   - `$ git push origin v0.1.2`

The rest of the process will take place in the github action: https://github.com/auth0/auth0-cli/actions/workflows/goreleaser.yml.
Once the workflow finishes, a new release will be available for the newly created tag.

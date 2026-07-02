## Dev Setup instructions

1. [Setup go](https://golang.org/doc/install)
2. Clone this repo: `git clone git@github.com:auth0/auth0-cli`
3. `make test` - ensure everything works correctly. Should see things pass.

### Adjusting your Development Environment (Optional)

To pass the integration tests, you must have the `AUTH0_DOMAIN`, `AUTH0_CLIENT_ID` and `AUTH0_CLIENT_SECRET` environment variable set. To get these values, you can:

1. Install [jq](https://jqlang.github.io/jq/)
2. [Setup a Machine-to-Machine application](https://auth0.com/docs/get-started/auth0-overview/create-applications/machine-to-machine-apps)
3. Use the resulting **Client Secret** values

#### Setting up your Environment Variables

You can set these variables in a `.env` file at the root of the project (replace the values with your own):

```shell
export AUTH0_DOMAIN="travel0.us.auth0.com"
export AUTH0_CLIENT_ID="tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka"
export AUTH0_CLIENT_SECRET="XXXXXXXXXXXXXXXXXXXXXXXXXXXX"
```

## Build and run on native platform

From the top-level directory:
```
$ make build
$ ./out/auth0 --help
```

## Adding a new command

This part is not fully fleshed out yet, but here are the steps:

1. Create a command (example: https://github.com/auth0/auth0-cli/blob/beta/internal/cli/login.go)
2. Add the command constructor to the root command: (e.g. somewhere here: https://github.com/auth0/auth0-cli/blob/beta/internal/cli/root.go)

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

The release is driven by a GitHub **workflow** triggered when a new **tag** is **created**. The workflow will run the checks and trigger _Goreleaser_ to:
- create the pre-release with the proper description (changelog)
- upload the binaries for the different architectures

> **Note:** Homebrew and Scoop are not updated for beta releases.

To release a new beta version:

1. pull the latest changes: 
   - `$ git checkout beta`
   - `$ git pull origin beta`
2. check the latest tag: 
   - `$ git fetch`
   - `$ git tag --list`
3. create the **new** beta tag. Beta releases use the format `vX.Y.Z-beta.N`. For example, if the latest tag is `v1.32.0-beta.1` and you want the next beta, create `v1.32.0-beta.2`:
   - `$ git tag v1.32.0-beta.2`
4. push the new tag: 
   - `$ git push origin v1.32.0-beta.2`

The rest of the process will take place in the github action: https://github.com/auth0/auth0-cli/actions/workflows/goreleaser.yml.
Once the workflow finishes, a new release will be available for the newly created tag.

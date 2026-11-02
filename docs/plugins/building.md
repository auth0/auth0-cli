---
layout: default
title: Building a plugin
parent: Plugins
nav_order: 3
---

In this page you create a plugin project, link a build of it to the Auth0 CLI, and run it as `auth0 <name>` while you work on it. The project comes from one of the starter projects that ship inside the CLI, in Go or in TypeScript.

## Before you start

A plugin is an executable plus a `plugin.json` file. The CLI runs the plugin only when you start it through the CLI, as `auth0 <name>`. If you run the executable by hand, it stops and tells you to use the CLI, because the variables that connect it to the CLI are not set:

```text
error: AUTH0_PLUGIN_DOMAIN is not set. This plugin has to be started by the Auth0 CLI: run `auth0 checkmate` instead of running the file directly
```

The commands for plugin authors, the `auth0 plugin dev` group, are available only in dev mode. Turn dev mode on in the shell you build in:

```sh
export AUTH0_CLI_PLUGIN_DEV_MODE=true
```

Only the exact value `true` turns dev mode on. Dev mode also makes the CLI read the links that `auth0 plugin dev link` creates. Without dev mode a linked build is ignored: `auth0 plugin list` shows nothing, and `auth0 checkmate` is an unknown command. Turn dev mode off on machines where you only use installed plugins, so a local build cannot run there.

Without dev mode, every `plugin dev` command stops with this message:

```text
===  error

 ▸    The commands for plugin authors are available only in dev mode. Turn it on with: AUTH0_CLI_PLUGIN_DEV_MODE=true.
```

You also need the toolchain of the language you pick: Go 1.26 or later, or Bun, a JavaScript runtime that can also compile a program into one executable. The CI of the starter projects uses Bun 1.4.4, and the commands in this page ran with Bun 1.4.4 and Go 1.26.

## Create a project

`auth0 plugin dev create` writes a new project into a directory. The directory is created when it does not exist, but its parent directory must exist. An empty directory is used as it is. A directory that holds files is refused, because an existing project is never overwritten.

```sh
auth0 plugin dev create checkmate --lang go --author "Platform team" --bugs https://github.com/acme/checkmate/issues
```

```text
 ▸    Created auth0/checkmate in /home/you/checkmate (27 files).
 ▸    Next:
  cd /home/you/checkmate
  go mod tidy && go build -o checkmate .
  AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link
  auth0 plugin approve checkmate
  auth0 checkmate
 ▸    To publish it, push the project to a new GitHub repository: the first push to main releases version 0.1.0. Release again with: make release BUMP=minor
```

The last line is the CLI's own output. A GitHub release alone does not make the plugin installable, because the registry catalog also has to list it, as a separate step. See [Releasing a plugin](releasing.md).

The path in this output is shortened. For a TypeScript project, pass `--lang ts`. The next steps then start with `bun install && bun run build`, and the release command is `bun run release minor`.

The name is the short name of the plugin. It starts with a lowercase letter and holds only lowercase letters, digits and hyphens. The plugin runs as `auth0 <name>`, so the name cannot be the name of a core command such as `apps`. The CLI refuses that name:

```text
===  error

 ▸    Auth0 already has a command named "apps". Plugin names cannot shadow core commands.
```

`--lang`, `--author` and `--bugs` are required. In a terminal, `create` asks for any value you leave out. Everything else takes a default. Without a terminal, a missing value is an error:

```text
===  error

 ▸    --lang is required: Language of the starter project: go or ts.
```

### Flags of `plugin dev create`

| Flag | Default | Meaning |
| --- | --- | --- |
| `--lang` | none, required | `go` or `ts`. |
| `--author` | none, required | The team or person that owns the plugin. It becomes `owner.team` in `plugin.json`. |
| `--bugs` | none, required | An HTTPS URL where people report problems. It becomes `owner.bugs`. |
| `--id` | `auth0/<name>` | The plugin ID. The CLI knows one registry, `auth0`, so the ID must be `auth0/` followed by the name. Another value is refused. |
| `--version` | `0.1.0` | The first version, as `MAJOR.MINOR.PATCH`. |
| `--description` | `A plugin for the Auth0 CLI` | One line, up to 120 characters. |
| `--command` | the name of the plugin | The name of the first command. When it equals the plugin name, it runs as `auth0 <name>`. Otherwise it runs as `auth0 <name> <command>`. |
| `--command-description` | the plugin description | What the first command does. |
| `--dir` | `./<name>` | Where to write the project. The directory must not exist or must be empty, and its parent must exist. |
| `--license` | `UNLICENSED` | Written into the files of the project. No `LICENSE` file is created. |
| `--scopes` | `read:clients` | Comma-separated Management API scopes for the first command. Pass `--scopes ""` for none. |
| `--from` | the starter projects in the CLI | A directory of your own starter projects, laid out like the built-in ones. See below. |
| `--json`, `--json-compact` | off | Print the directory, the files, the ID, the language and any warnings as JSON. |

With no scopes, `create` still writes the project and warns you that the sample call will be denied:

```text
 ▸    The first command declares no scopes, so the Management API call of the sample is denied. Add the scopes it needs under permissions.scopes in plugin.json.
```

When the CLI that runs `create` is a release build, the manifest also gets `engines.auth0-cli`, set to `>=` followed by the version of that CLI. A development build writes no `engines` block.

### Your own starter projects

`--from <dir>` replaces the built-in projects with your own. The directory holds a `go` folder, a `ts` folder, or both, and each one is a complete project. A file named `X.tmpl` is written as `X`. The text `__PLUGIN_NAME__`, `__COMMAND__` and `__LICENSE__` is replaced in every file. A tree may not hold symbolic links, more than 64 files, or more than 256 KB, and it may not hold a `plugin.json`, because `create` always writes one. `create` checks all of this before it writes anything.

## The project layout

The Go project has 27 files, and the TypeScript project has 31. These are the parts you will change.

Go:

```text
checkmate/
  main.go                  starts the command
  commands/
    registry.go            finds the command from the arguments
    clients.go             the sample command
  plugin/
    client.go              the Management API client, pointed at the CLI proxy
    output.go              JSON, table and message helpers
    errors.go              error text and exit codes
    paginate.go            walks every page of a list call
    plugin.go              the name, the version and the exit codes
  scripts/release/         the program behind make release
  plugin.json              the manifest
  Makefile                 build, test, lint, release
  .goreleaser.yaml         builds and packs the release archives
  .github/                 CI and release workflows
```

TypeScript:

```text
checkmate/
  src/
    index.ts               the entry point
    run.ts                 finds the command and turns errors into exit codes
    commands/
      index.ts             the list of commands
      clients.ts           the sample command
    lib/                   environment, SDK client, output, errors, flags, pagination
  scripts/release.ts       the program behind bun run release
  scripts/version.ts       the version rules of the release program
  plugin.json              the manifest
  package.json             build, test, typecheck and release scripts
  .github/                 CI and release workflows
```

Both projects include tests. The Go project calls the Management API with [go-auth0](https://github.com/auth0/go-auth0). The TypeScript project uses the [auth0](https://www.npmjs.com/package/auth0) Node package and compiles to one executable with Bun.

### The manifest

`create` writes `plugin.json` from the values you gave, and it validates the file before it writes anything. This is the manifest of the Go project above:

```json
{
  "$schema": "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/main/schema/plugin.v1.schema.json",
  "id": "auth0/checkmate",
  "version": "0.1.0",
  "apiVersion": 1,
  "description": "A plugin for the Auth0 CLI",
  "owner": {
    "team": "Platform team",
    "bugs": "https://github.com/acme/checkmate/issues"
  },
  "platforms": [
    "darwin-arm64",
    "darwin-x64",
    "linux-x64",
    "linux-arm64",
    "linux-x64-musl",
    "linux-arm64-musl",
    "windows-x64"
  ],
  "commands": [
    {
      "name": "checkmate",
      "description": "A plugin for the Auth0 CLI",
      "interactive": false,
      "timeout": "5m",
      "output": {
        "json": true,
        "csv": false,
        "agentMode": true
      },
      "permissions": {
        "scopes": [
          "read:clients"
        ],
        "hosts": [],
        "files": [],
        "env": []
      }
    }
  ]
}
```

Every field is described in the [reference](reference.md#pluginjson).

## Build, link and run

Build the plugin so that the executable sits next to `plugin.json`, named like the plugin.

For Go:

```sh
cd checkmate
go mod tidy
go build -o checkmate .
```

`go mod tidy` also writes `go.sum`. For TypeScript:

```sh
cd checkmate
bun install
bun run build
```

The TypeScript build writes `./checkmate` for the machine you are on.

Then link the directory to the CLI:

```sh
auth0 plugin dev link
```

```text
 ▸    Linked auth0/checkmate 0.1.0 from /home/you/checkmate. Run it with: auth0 checkmate
 ▸    Every run prints this line on stderr: Running the linked build of auth0/checkmate 0.1.0 from /home/you/checkmate. Its files are not checked against a release.
 ▸    auth0/checkmate is not approved on any tenant. Log in with auth0 login, then approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant <tenant>
```

`link` takes a directory as its argument and defaults to the current one. The directory must hold a valid `plugin.json`, and the executable is the file named like the plugin in it, with `.exe` added on Windows. If your build lands somewhere else, pass its path with `--bin`:

```sh
auth0 plugin dev link ./checkmate --bin ./checkmate/dist/checkmate
```

`link` runs the same manifest checks as `plugin dev validate`, and it refuses a manifest with a problem with the reason `plugin_link_invalid`. It also refuses a missing executable.

The link is stored in `dev-links.json` in the plugins directory, which is listed in the [paths](reference.md#paths). The CLI reads `plugin.json` and the executable again on every run, so after a change you only rebuild. You link once. Linking the same plugin again replaces the link, and `auth0 plugin dev unlink checkmate` removes it. The build itself is never touched.

A linked plugin takes the place of an installed plugin with the same name. `auth0 plugin list` marks it:

```text
  ID               VERSION  PINNED  PREVIOUS  SIZE  LINKED  
  auth0/checkmate  0.1.0    no                0 B   yes     
```

A linked plugin is held to the same rules as an installed one: the same proxy, the same scope checks, and the same approval by a person. The checks that make no sense for an unpublished build are skipped. Those are the hashes of its files, the lockfile and the revocation list.

### Approve and run

A plugin runs on a tenant only after a person approves its permissions there. Approval needs a terminal and a tenant:

```sh
auth0 plugin approve checkmate --tenant example.us.auth0.com
```

The screen shows what the plugin asks for, and your answer is saved for that tenant:

```text
 ▸    auth0/checkmate 0.1.0  A plugin for the Auth0 CLI

This plugin asks to use your Auth0 tenant example.us.auth0.com:

  Can read     clients
  Interactive  no

Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI.

Approve for example.us.auth0.com? [y/N/d]  (d shows the exact scopes)
```

`approve` refuses to run without a terminal, in agent mode and with `--no-input`. It reports that only a person can approve a plugin, and it prints the command to run in a terminal:

```text
===  example.us.auth0.com error

 ▸    Only a person can approve a plugin. Run this in a terminal: auth0 plugin approve auth0/checkmate --tenant example.us.auth0.com.
```

Approvals are kept per tenant, so the command also needs a tenant. Pass `--tenant`, or use the default tenant.

Before the CLI starts any plugin, linked or installed, it needs a registry index that it can verify. The index carries the table that says which Management API calls each scope allows, and the list of secret fields that the proxy removes from responses. A linked build is not in the index, but it is checked against the same table. If the CLI cannot verify an index, the plugin does not start:

The output below is from the current build, which does not embed a trusted registry key yet.

```text
 ▸    Running the linked build of auth0/checkmate 0.1.0 from /home/you/checkmate. Its files are not checked against a release.

===  example.us.auth0.com error

 ▸    Checkmate 0.1.0 was not run, because the registry index cannot be trusted: no trusted registry keys are embedded in this build. Reconnect and run: auth0 plugin search.
```

With `--json`, the same failure has the reason `plugin_registry_unavailable`. The CLI needs a trusted registry key to check the registry, so builds without one stop here. Creating, building, validating, linking and approving a plugin do not need the index. See [Status](index.md#status).

Once a build can verify the index, `auth0 checkmate` runs the sample command. It lists the clients of the tenant and needs the `read:clients` scope, which the manifest declares.

### The development loop

1. Change the code and rebuild.
2. Run `auth0 <name>` again. There is nothing to reinstall or relink.
3. If you change `permissions`, whether a command is `interactive`, or the name of a command, the CLI asks for approval again. The approval is keyed on a hash of those fields only. Descriptions, flags, timeouts, output settings and the version do not change the hash.

You can see the hash that the CLI computes from your manifest with `auth0 plugin info checkmate`. This is the output after the approval above:

```text
  ID                auth0/checkmate
  VERSION           0.1.0
  DESCRIPTION       A plugin for the Auth0 CLI
  OWNER             Platform team https://github.com/acme/checkmate/issues
  REQUIRES          none
  PLATFORMS         darwin-arm64, darwin-x64, linux-x64, linux-arm64, linux-x64-musl, linux-arm64-musl, windows-x64
  PERMISSIONS       1 scope, 0 hosts, 0 file paths, 0 environment variables
  PERMISSIONS HASH  sha256:daccec04e517c367feb51cd7a87fcf4933f05f0d1cdd29a2fe4091b52d7a31e9
  VERSIONS
  INSTALLED         no
  APPROVED TENANTS  example.us.auth0.com
  LINKED            yes, a local build runs in place of any installed copy
```

After you add `read:users` to the scopes, the same command shows `2 scopes` and a different hash. That is why a changed scope needs a new `plugin approve`. Changing only the description or the timeout leaves the hash as it was.

## How the CLI starts your plugin

The sections below describe what the CLI does between your command line and your `main`, so you can tell where a problem comes from.

### Arguments

The CLI removes its own flags from the arguments wherever they appear: `--tenant`, `--debug`, `--no-input`, `--no-color`, `--agent-mode`, `--json`, `--json-compact`, `--csv` and `--help`. Everything else reaches your plugin exactly as typed, in the original order, and the CLI does not check it. Everything after a bare `--` goes to your plugin untouched, including a later `--tenant`.

A manifest cannot declare a flag with one of those names, and it cannot use `-h` as a shorthand. `plugin dev validate` reports both.

The flags you list under `flags` in `plugin.json` appear in `auth0 <name> --help` and in shell completion. The CLI does not parse or check them. Your code does that.

`auth0 <name> --help` and `-h` are answered by the CLI from the manifest, and they never reach your plugin. For the sample manifest the help starts like this:

```text
A plugin for the Auth0 CLI

Management API scopes: read:clients
Timeout: 5m
Output: --json, --json-compact, agent mode

...
```

A command that is not the root command runs as `auth0 <name> <command>`. The CLI passes the command name to your plugin as its first argument. Type the command name directly after the plugin name. If a flag comes first, the CLI may not find the command. A call of the root command that has the name of one of its subcommands among its arguments, before a bare `--`, is refused with the reason `plugin_ambiguous_subcommand`, because it is not clear which command was meant. Write `--flag=value`, or put the argument after `--`.

### Environment

The plugin gets a small environment, and nothing else is inherited. These variables are set for every run:

| Variable | Meaning |
| --- | --- |
| `AUTH0_PLUGIN_DOMAIN` | Address of the CLI proxy, as `host:port`. It is not the tenant domain. |
| `AUTH0_PLUGIN_MANAGEMENT_TOKEN` | A secret for this run only, which lets the plugin use the proxy. It is not an Auth0 token. |
| `AUTH0_PLUGIN_CA_CERT` | A PEM file with the certificate of the proxy. |
| `SSL_CERT_FILE`, `NODE_EXTRA_CA_CERTS` | The same certificate file, for runtimes that read these variables. |
| `AUTH0_PLUGIN_TENANT` | The tenant the run is for. |
| `AUTH0_PLUGIN_JSON`, `AUTH0_PLUGIN_JSON_COMPACT`, `AUTH0_PLUGIN_CSV` | `true` or `false`, from the output flags. Agent mode turns `AUTH0_PLUGIN_JSON` on. |
| `AUTH0_PLUGIN_AGENT_MODE`, `AUTH0_PLUGIN_NO_INPUT`, `AUTH0_PLUGIN_NO_COLOR`, `AUTH0_PLUGIN_DEBUG` | `true` or `false`, from the global flags. |

Besides these, the plugin gets a short list of variables from your own environment, such as `PATH`, `HOME` and `TERM`, and the variables that its command declares under `permissions.env`. Names that start with `AUTH0_` are never passed through, because they belong to the CLI and its credentials. The [reference](reference.md#environment-variables-your-plugin-receives) has the complete list.

### Input, output and exit codes

Your plugin writes results to stdout and everything else to stderr. Both go straight through to the terminal. Standard input is passed on when the command is `interactive` or when standard input is not a terminal, for example when something is piped in. Otherwise the plugin gets the null device. The plugin starts in the directory where you ran `auth0`.

The CLI does not pass the exit code of your plugin on. A non-zero code makes `auth0` exit with 1 and report the reason `plugin_exit`. The JSON error carries the code your plugin used, as `exitCode` in its details. The starter projects use 0 for success, 1 when the command fails and 2 when the arguments are wrong.

### Timeouts and stopping

A command has a `timeout` in `plugin.json`, which is a Go duration such as `30s` or `10m`. The starter projects set `5m`. A command with no timeout gets the default of the registry index, which is 10 minutes unless the index says otherwise. When the time is up, or when the user presses Ctrl-C, the CLI stops the plugin and everything it started:

- On macOS and Linux, a plugin that is not interactive runs in its own process group. The CLI sends `SIGTERM` to the group, and `SIGKILL` if anything is still running after 5 seconds.
- On Windows, a plugin that is not interactive runs in a job object, which is a group of processes that Windows can end together. The CLI sends a Ctrl-Break event, which is best effort, and ends the job after 5 seconds.
- An interactive plugin stays in the process group of the CLI, so that the terminal keeps working. On macOS and Linux, a timeout then stops only the plugin process itself.

Both starter projects cancel their requests on Ctrl-C and `SIGTERM`. A run that goes over its timeout ends with the reason `plugin_timeout`. A run that was interrupted ends with `plugin_interrupted`.

## Call the Management API

Your plugin never receives a real access token. It calls the CLI proxy instead, and the proxy replaces the per-run secret with the real token.

The client is set up in one file of each starter project: `plugin/client.go` in Go, and `src/lib/env.ts` with `src/lib/client.ts` in TypeScript. The client uses the address in `AUTH0_PLUGIN_DOMAIN` and the secret in `AUTH0_PLUGIN_MANAGEMENT_TOKEN`, and it trusts the certificate in `AUTH0_PLUGIN_CA_CERT`. The Go client loads that certificate itself, because Go ignores `SSL_CERT_FILE` on macOS and Windows. Bun reads `NODE_EXTRA_CA_CERTS`, so the TypeScript project needs no extra code for it.

The proxy applies these rules to every call:

- It forwards only calls under `/api/v2/`, the Management API.
- It forwards a call only when a scope that the invoked command declared allows it. A command that declares no scopes cannot call the API.
- It uses the scopes of the command that was run, not the union of the scopes of all commands in the manifest.
- It removes secret fields, such as client secrets, from the responses.
- It refuses a call that is not in its scope table, and a path that is not in canonical form.

When a call needs a scope that your command did not declare, the proxy answers with HTTP 403 and an `errorCode` of `insufficient_scope`. The message names the scope:

```text
Plugin auth0/checkmate did not declare scope "read:clients" required for GET /api/v2/clients. Add it to permissions.scopes in plugin.json.
```

The starter projects print `error: the CLI blocked this call.`, then that message, then the command that approves the new permissions, `auth0 plugin approve <name>`. After the run, the CLI also prints one summary line on stderr. It counts the calls that were denied and says which permissions to review. The [reference](reference.md#proxy-error-codes) lists every error code that the proxy returns.

If the session is a user login, the proxy cannot renew an expired token without the user. Your plugin then gets the error code `session_expired`. A session made with client credentials is renewed for you, up to three times in one run.

## Scopes and permissions

Each command declares what it needs under `permissions` in `plugin.json`:

| Field | Enforced by the CLI | What it does |
| --- | --- | --- |
| `scopes` | Yes | The Management API scopes the command may use. The proxy allows a call only if one of these scopes allows it. |
| `env` | Yes | Names of environment variables from the user's environment that the command may read. Names that start with `AUTH0_` are refused. |
| `hosts` | No | Hosts the plugin says it contacts. Each one needs a `reason`. |
| `files` | No | Paths the plugin says it reads or writes. Each one needs `access` (`read` or `write`) and a `reason`. |

Hosts and files are declarations by the publisher. The consent screen says so. A plugin runs as a normal process with the permissions of the user, so it can reach hosts and files that it did not declare. Write honest reasons, because the person who approves the plugin reads them.

The consent screen shows the union of the permissions of all commands. At run time, the proxy enforces only the scopes of the command that runs.

## Add a command

For Go:

1. Copy `commands/clients.go` to `commands/<name>.go`. Change the name, the help text and the code in the run function, and keep the call to `Register` in `init`.
2. Add the command to `commands` in `plugin.json`, with its description and the scopes it needs.
3. Rebuild.

For TypeScript:

1. Copy `src/commands/clients.ts` to `src/commands/<name>.ts`. Change the name, the help text and the code in `run`.
2. Import it in `src/commands/index.ts` and add it to the list.
3. Add the command to `commands` in `plugin.json`, then rebuild.

A command whose name is not the name of the plugin runs as `auth0 <name> <command>`. Declare its flags in the code and list them under `flags` in `plugin.json`. Both starter projects read `--json` and the other output settings from the environment, so the command only chooses between printing JSON and a table.

### Output modes

A command states which output modes it supports, under `output`:

- `output.json: true` allows `--json` and `--json-compact`.
- `output.csv: true` allows `--csv`.
- `output.agentMode: true` allows the command to run in agent mode.

When a user asks for a mode that the command did not declare, the CLI refuses before it starts the plugin, with the reason `plugin_output_unsupported`. A manifest with no `output` block declares nothing, so `--json` and agent mode are both refused. The starter projects declare `json` and `agentMode`.

### Interactive commands

A command with `interactive: true` needs a terminal. The CLI refuses it with the reason `plugin_interactive` when `--no-input` is set, in agent mode, and when standard input or output is not a terminal.

## Check the manifest

`auth0 plugin dev validate` checks `plugin.json` the way the registry checks a plugin when it is published. It works offline, takes a directory or a file (the default is the current directory), and exits with 1 when it finds a problem.

```sh
auth0 plugin dev validate
```

```text
 ▸    The plugin manifest is valid.
```

It prints every problem, with the path of the field, not only the first one. Problems against the JSON schema come first. The checks that the schema cannot express run only when the schema check passes, so fix the fields that are listed and run the command again.

```text
  PATH                                 PROBLEM
  commands[0].flags[0].name            flag "json" collides with a flag owned by the CLI
  commands[0].permissions.env[0].name  "AUTH0_DOMAIN" uses a prefix reserved by the CLI (AUTH0_)

===  error

 ▸    The plugin manifest has 2 problems.
```

A schema problem reads like this. The long patterns are shortened here with "...":

```text
  PATH                 PROBLEM
  commands[0].timeout  'soon' does not match pattern '^([0-9]+(\\.[0-9]+)?(ns|us|ms|s|m|h))+$'
  owner.bugs           'http://example.com/issues' does not match pattern '^https://...'
  version              'v1.0' does not match pattern '^(0|[1-9][0-9]*)\\....'
```

The CLI also refuses a plugin ID whose name is the name of a core command:

```text
  PATH  PROBLEM
  id    auth0 already has a command named "apps". Plugin names cannot shadow core commands
```

With `--bin <file>`, the command also checks that the executable exists and can run. With `--json`, the result is printed on stdout. A valid manifest and an invalid one have the same shape:

```json
{
    "valid": false,
    "problems": [
        {
            "path": "--bin",
            "message": "stat ./checkmate: no such file or directory"
        }
    ]
}
```

On failure the process exits with 1, and the error carries the reason `plugin_manifest_invalid`. With `--json`, that error is printed on stderr, and its `details.problems` list holds the same problems as the `problems` list on stdout.

## Run the checks of the starter project

The starter projects have tests and a lint or typecheck step. For Go, run `make test` and `make lint`. For TypeScript, run `bun test` and `bun run typecheck`. On a new project, all of them pass. The CI of the starter runs the tests and the build on Linux, macOS and Windows for every pull request, and again before every release.

## Next

- [Releasing a plugin](releasing.md) explains the release command and the release workflow of the starter project.
- [Reference](reference.md) lists every field of `plugin.json`, every variable, every path and every reason code.
- [Security and trust model](security.md) explains what the CLI protects and what it cannot.

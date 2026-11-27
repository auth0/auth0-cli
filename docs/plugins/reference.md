---
layout: default
title: Reference
parent: Plugins
nav_order: 9
---

This page lists every field of `plugin.json`, the environment variables that the CLI and your plugin read, the files the CLI keeps on disk, the platform names, the reason codes and the error codes of the proxy. Use it when you need an exact name or value.

## plugin.json

The manifest is a JSON object for `apiVersion` 1. The commands `plugin dev create`, `plugin dev validate` and `plugin dev link` check it strictly: a field that the schema does not list is an error. The CLI reports a value that breaks a rule with the path of the field. The fields are listed below.

### Top level

| Field | Required | Rules |
| --- | --- | --- |
| `$schema` | No | A string. The URL of the schema that the manifest was written against. |
| `id` | Yes | `registry/name`, in lowercase. Each part holds lowercase letters, digits and hyphens, and starts with a letter or a digit. The CLI knows one registry, `auth0`, so the ID is `auth0/<name>`. The name cannot be the name of a core command such as `apps`. |
| `version` | Yes | A semantic version without a leading `v`, such as `1.2.0`. The schema also accepts a prerelease or build suffix. The CLI installs only `MAJOR.MINOR.PATCH`. |
| `apiVersion` | Yes | The integer `1`. |
| `description` | Yes | One line of 1 to 120 characters, with no line breaks. |
| `homepage` | No | An HTTPS URL of the home page of the plugin. |
| `owner` | Yes | An object with `team` and `bugs`. No other key is allowed. |
| `owner.team` | Yes | The team or person that owns the plugin. At least one character. |
| `owner.bugs` | Yes | An HTTPS URL where people report problems. |
| `engines` | No | An object with one key, `auth0-cli`. |
| `engines.auth0-cli` | Yes, inside `engines` | One or more comparators separated by spaces, such as `>=1.39.0 <2.0.0`. Each comparator is `>=`, `<=`, `>`, `<` or `=` followed by `MAJOR.MINOR.PATCH`. All of them must match. The CLI compares the three numbers and ignores a prerelease suffix. |
| `platforms` | Yes | A list of at least one platform name, with no repeats. See [Platforms](#platforms). |
| `commands` | Yes | A list of at least one command. |

### Command

| Field | Required | Rules |
| --- | --- | --- |
| `name` | Yes | Lowercase letters, digits and hyphens, starting with a letter. A command with the name of the plugin is the root command, and it runs as `auth0 <name>`. Any other command runs as `auth0 <name> <command>`. Names are unique within the manifest. |
| `description` | Yes | At least one character. Shown in the help of the command. |
| `flags` | No | A list of flags. See [Flag](#flag). |
| `interactive` | No | `true` when the command needs a terminal. The default is `false`. |
| `timeout` | No | A Go duration greater than zero, such as `30s` or `10m`. When it is missing, the default of the registry index applies, which is `10m` unless the index sets another value. |
| `output` | No | An object with the booleans `json`, `csv` and `agentMode`. Only the modes that are `true` are allowed. A missing `output` allows no mode. |
| `permissions` | No | An object with `scopes`, `hosts`, `files` and `env`. See [Permissions](#permissions). |

### Flag

| Field | Required | Rules |
| --- | --- | --- |
| `name` | Yes | The same form as a command name. It cannot be a flag that the CLI owns: `tenant`, `json`, `json-compact`, `csv`, `agent-mode`, `no-input`, `no-color`, `debug` or `help`. Names are unique within the command. |
| `shorthand` | No | One ASCII letter, used as `-x`. It cannot be `h`, and it is unique within the command. |
| `type` | Yes | One of `string`, `bool`, `int`, `float` or `stringArray`. |
| `description` | Yes | At least one character. |

The CLI does not parse the flags of a plugin. It only shows them in the help and in shell completion. Your code reads them.

### Permissions

| Field | Required | Rules |
| --- | --- | --- |
| `scopes` | No | A list of Management API scopes, with no repeats. Each one is lowercase letters, a colon, then lowercase letters, digits or underscores, such as `read:clients`. The proxy uses them. |
| `hosts` | No | A list of objects with `host` and `reason`. `host` is a lowercase host name with no scheme, port or path. A single `*.` at the start is allowed, and it must be followed by at least two labels, such as `*.amazonaws.com`. |
| `files` | No | A list of objects with `path`, `access` and `reason`. `access` is `read` or `write`. |
| `env` | No | A list of objects with `name` and `reason`. `name` is an uppercase environment variable name. A name that starts with `AUTH0_`, in any case, is refused. |

Every `reason` and every `path` is free text. It must have at least one character, and it cannot hold control characters or the Unicode line and paragraph separators. The consent screen shows the reasons, so write them for the person who approves the plugin.

Hosts and files are declared, not enforced. Scopes and environment variables are enforced: the proxy checks scopes at run time, and only the declared environment variables reach the plugin.

### What the approval covers

The approval of a plugin is keyed on a hash of the permissions of all commands, including their reasons, and of each command's name and `interactive` value. `auth0 plugin info` shows this hash as `PERMISSIONS HASH`. Changing any of these values asks for a new approval. Changing `description`, `flags`, `timeout`, `output` or `version` does not.

## Platforms

The platform names in `platforms` are these seven. The CLI picks the platform for the machine that runs it.

| Name | Operating system | Architecture | C library |
| --- | --- | --- | --- |
| `darwin-arm64` | macOS | arm64 | Not applicable |
| `darwin-x64` | macOS | x64 | Not applicable |
| `linux-x64` | Linux | x64 | glibc |
| `linux-arm64` | Linux | arm64 | glibc |
| `linux-x64-musl` | Linux | x64 | musl |
| `linux-arm64-musl` | Linux | arm64 | musl |
| `windows-x64` | Windows | x64 | Not applicable |

On Linux, the CLI checks which C library the system uses, and it picks the `musl` names on a musl system such as Alpine. The CLI installs `.tar.gz` and `.zip` archives. The starter projects pack Windows archives as `.zip` and the others as `.tar.gz`.

## Paths

The CLI keeps its settings and plugins in the directory `~/.config/auth0` on macOS and Linux. It finds the home directory from the `HOME` environment variable on every system. The paths below are relative to that directory.

| Path | What it holds |
| --- | --- |
| `config.json` | The tenants and the default tenant. |
| `plugins/` | The plugin store. The directory is created with owner-only permissions. |
| `plugins/plugins.lock.json` | The installed plugins: the active version, the previous version, the pinned version and the hashes of the binary, the manifest and the archive. |
| `plugins/approvals.json` | The approvals of each plugin, per tenant. The file is readable and writable by the owner only. |
| `plugins/dev-links.json` | The local builds that `plugin dev link` linked: the directory, the executable and, when `--policy` was given, the dev policy file. |
| `dev-policy.json` | Not in this directory. It sits next to `plugin.json` in the directory of a linked build, and holds the scope rules of that build. Only a linked build in dev mode reads it. See [Building a plugin](building.md#the-dev-policy). |
| `plugins/registries/auth0/` | The cache of the registry: `metadata/` with the latest `root.json`, `timestamp.json`, `snapshot.json` and `targets.json`, `roots/` with the roots after the first one (`<N>.root.json`, from 2 up), `index.json`, `state.json` with the newest index version accepted and the time of the last refresh, and the lock file `.lock` that guards them. The first root is built into the CLI and is not stored. |
| `plugins/store/<registry>/<name>/<version>/` | The files of an installed version: the executable and `plugin.json`. For example, `plugins/store/auth0/checkmate/1.0.0/`. |
| `plugins/tmp/` | The staging area for downloads and installs. |
| `plugins/hint.json` | The time of the last update hint. |
| `plugins/.lock`, `.approvals.lock`, `.dev-links.lock` | Lock files that stop two CLI processes from changing the same file at the same time. |

On Windows, the CLI builds the same paths from the `HOME` environment variable. If `HOME` is not set, the paths are relative to the current directory. Set `HOME` before you use the plugin commands on Windows.

## Environment variables your plugin receives

The CLI builds the environment of a plugin from three parts. Nothing else from your environment reaches the plugin.

**Set by the CLI for every run:**

| Variable | Value |
| --- | --- |
| `AUTH0_PLUGIN_DOMAIN` | The address of the proxy, as `host:port`. It is not the tenant domain. |
| `AUTH0_PLUGIN_MANAGEMENT_TOKEN` | A secret for this run only. The proxy accepts it instead of a token. |
| `AUTH0_PLUGIN_CA_CERT` | The path of a PEM file with the certificate of the proxy. |
| `SSL_CERT_FILE` | The same file as `AUTH0_PLUGIN_CA_CERT`. |
| `NODE_EXTRA_CA_CERTS` | The same file as `AUTH0_PLUGIN_CA_CERT`. |
| `NO_PROXY`, `no_proxy` | The value from your environment, with the loopback addresses added, so that calls to the proxy do not go through a proxy of your own. |
| `AUTH0_PLUGIN_TENANT` | The tenant that the run is for. |
| `AUTH0_PLUGIN_JSON`, `AUTH0_PLUGIN_JSON_COMPACT`, `AUTH0_PLUGIN_CSV` | `true` or `false`, from the output flags. In agent mode, `AUTH0_PLUGIN_JSON` is `true`. |
| `AUTH0_PLUGIN_AGENT_MODE`, `AUTH0_PLUGIN_NO_INPUT`, `AUTH0_PLUGIN_NO_COLOR`, `AUTH0_PLUGIN_DEBUG` | `true` or `false`, from the global flags. |

**Copied from your environment, when set and not empty:**

| Variable | Note |
| --- | --- |
| `PATH`, `HOME`, `LANG`, `LC_ALL`, `LC_CTYPE`, `TERM`, `TMPDIR` | The basic settings of a process. |
| `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, `http_proxy`, `https_proxy`, `no_proxy` | Proxy settings of your network. |
| `SystemRoot`, `TEMP`, `TMP`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `PATHEXT`, `ComSpec` | Windows only. |

**Declared by the command:** every name in `permissions.env` is copied from your environment when it is set and not empty. A name that starts with `AUTH0_` is never copied, even when the manifest asks for it.

When a name is set both by the CLI and by your environment, the CLI's value wins.

## Environment variables the CLI reads

| Variable | Effect |
| --- | --- |
| `AUTH0_CLI_PLUGIN_DEV_MODE` | Turns on the `auth0 plugin dev` commands and makes the CLI read the links of `plugin dev link`. Only the exact value `true` turns it on. |
| `AUTH0_CLI_PLUGINS_DISABLED` | A value of `true`, or any value that is not a boolean, turns the plugin commands off for install, update, approve and running a plugin. Search, list, info and uninstall still work. |
| `AUTH0_CLI_PLUGIN_APPROVALS` | Pins the approved permissions of plugins for CI. The value is a comma-separated list of `registry/name@sha256:<64 lowercase hex digits>`, where the hash is the permissions hash that `auth0 plugin info` shows. A pin is used in agent mode too. The CLI never writes a pin to disk. A malformed entry is an error. |
| `AUTH0_CLI_PLUGIN_UPDATE_HINT` | `false`, or any value that is not a boolean, turns off the one-line hint about a newer version. The hint is shown at most once a day, on stderr, after a plugin ran. |
| `AUTH0_AGENT_MODE` | `true` or `false`. It overrides the detection of agent mode. The `--agent-mode` flag wins over it. |
| `AUTH0_CLI_ANALYTICS` | `false` turns off usage tracking, including the events of the plugin commands. |
| `AUTH0_CLI_AUTH_MODE`, `AUTH0_DOMAIN`, `AUTH0_CLIENT_ID`, `AUTH0_CLIENT_SECRET`, `AUTH0_API_TOKEN` | Environment login. With `AUTH0_CLI_AUTH_MODE=env`, the plugin commands use `AUTH0_DOMAIN` as the tenant when you pass no `--tenant`. |

## Author commands

These commands need `AUTH0_CLI_PLUGIN_DEV_MODE=true`. [Building a plugin](building.md) walks through them.

| Command | Flags | What it does |
| --- | --- | --- |
| `auth0 plugin dev create <name>` | `--lang`, `--author`, `--bugs`, `--id`, `--version`, `--description`, `--command`, `--command-description`, `--dir`, `--license`, `--scopes`, `--from`, `--json`, `--json-compact` | Writes a new project. It includes a starter `dev-policy.json` that is derived from `--scopes`: `read:X` adds `GET /api/v2/X` and `GET /api/v2/X/{id}`, `create:X` adds `POST /api/v2/X`, `update:X` adds `PATCH /api/v2/X/{id}`, and `delete:X` adds `DELETE /api/v2/X/{id}`. An underscore in `X` becomes a hyphen. |
| `auth0 plugin dev validate [path]` | `--bin`, `--json`, `--json-compact` | Checks `plugin.json`. If a `dev-policy.json` sits next to it, the command also checks that the file is valid, that every scope a command declares has a rule, and that every rule allows at least one declared scope. A missing file is not a problem. |
| `auth0 plugin dev link [path]` | `--bin`, `--policy`, `--json`, `--json-compact` | Links a local build. `--policy <file>` names the dev policy to use instead of `dev-policy.json` in the plugin directory. A relative path is made absolute, and the link stores it. The command checks the policy when it links. A directory with no `dev-policy.json` is linked with a warning, and the plugin does not run until the file exists. |
| `auth0 plugin dev unlink <name>` | | Removes the link. |

The format of `dev-policy.json` is a `rules` list of `method`, `path` and `anyOf`, and an optional `secretFields` list. [The dev policy](building.md#the-dev-policy) describes it.

## Exit codes and errors

A failed plugin command exits with 1, including a run in which the plugin itself failed. An interrupt, such as Ctrl-C, exits with 130 instead. When the plugin exits with a non-zero code, the CLI does not pass that code on. It is in the error details as `exitCode`.

With `--json`, a failure prints one object. The example below is the failure of an installed plugin in the current build, which does not embed a trusted registry root yet:

```json
{"error":{"code":"validation","reason":"plugin_registry_unavailable","message":"checkmate 0.1.0 was not run, because the registry index cannot be trusted: no trusted registry keys are embedded in this build. Reconnect and run: auth0 plugin search"}}
```

The `code` is a class of failure: `usage`, `validation`, `auth`, `not_found`, `plugin` or `unknown`. The `reason` says which failure it was. A plugin run that fails also carries `details` with `pluginId`, `version`, `exitCode` (left out when the plugin did not exit by itself) and `deniedCalls` (left out when the proxy denied nothing).

## Reason codes

| Reason | Class | Meaning |
| --- | --- | --- |
| `plugin_dev_mode_required` | validation | A `plugin dev` command ran without `AUTH0_CLI_PLUGIN_DEV_MODE=true`. |
| `plugin_create_invalid` | validation | `plugin dev create` refused the input: a missing value, a name or URL that is not valid, an ID that is not `auth0/<name>`, a directory that is not empty, a parent directory that does not exist, or a `--from` tree that is not allowed. |
| `plugin_manifest_invalid` | validation | `plugin dev validate` found problems in the manifest or in `dev-policy.json`, or the `--bin` file cannot run. |
| `plugin_link_invalid` | validation | `plugin dev link` refused the directory: the manifest is not valid, the executable is missing, the ID is not `auth0/<name>`, or the dev policy cannot be used (a file given with `--policy` that is missing or exists but is not valid, or a `dev-policy.json` that is not valid). |
| `plugin_dev_policy_missing` | validation | A linked plugin was run, and there is no dev policy at the path that the link uses. The plugin did not start. |
| `plugin_dev_policy_invalid` | validation | A linked plugin was run, and its dev policy cannot be used: it cannot be read, is too large, is not the JSON the CLI reads, or has rules or secret field names that are not valid. The plugin did not start. |
| `plugin_not_linked` | validation | `plugin dev unlink` was given a plugin that is not linked. |
| `plugin_name_reserved` | validation | The name of the plugin is the name of a core command. |
| `plugin_reference` | validation | The plugin reference is not valid, or it has a version where only a name is allowed. |
| `plugin_update_target` | validation | `plugin update` got neither a plugin name nor `--all`, or it got both. |
| `plugin_not_installed` | validation, or `not_found` | The plugin named in the command is not installed. When a command name matches a plugin of the saved registry index that is not installed, the class is `not_found`, and the details name the install command. |
| `plugin_no_tenant` | validation | The command has no tenant. Pass `--tenant`, or log in. |
| `plugin_tenant_mismatch` | validation | The session belongs to another tenant than the one the plugin is approved for. |
| `plugin_command_missing` | validation | The command is not in the manifest of the installed or linked plugin. The manifest changed since the run started. |
| `plugin_changed` | validation | The plugin changed while the session was being opened. For a linked build this includes `dev-policy.json`, which must be the file that was approved. Run it again. |
| `plugin_verification_failed` | validation | The installed files do not match what was installed. Reinstall the plugin. |
| `plugin_registry_unavailable` | validation | The registry index cannot be verified or cannot be used, so the installed plugin did not start. A linked build does not use the registry index. |
| `plugin_revoked` | validation | The installed version was revoked in the registry. Update the plugin. |
| `plugin_interactive` | validation | An interactive command ran with `--no-input`, in agent mode, or without a terminal. |
| `plugin_output_unsupported` | validation | The command was asked for an output mode that its manifest does not declare. |
| `plugin_not_approved` | validation | The plugin is not approved for the tenant, or its permissions changed since the last approval. For a linked build, a change to `dev-policy.json` counts too. |
| `plugin_approvals_damaged` | validation | The approvals file cannot be read. |
| `plugin_approval_check` | validation | The approvals could not be checked. |
| `plugin_timeout_invalid` | validation | The timeout of the command cannot be used. |
| `local_validation` | validation | A value that the CLI reads from the environment or the config cannot be used. For example, an entry of `AUTH0_CLI_PLUGIN_APPROVALS` is malformed. The message names the value. |
| `plugin_ambiguous_subcommand` | usage | The root command was called with the name of one of its subcommands among its arguments, before a bare `--`. Put the command name first, or write the value as `--flag=value`. |
| `plugin_session_unavailable` | auth | The run has no login session. Run `auth0 login`. |
| `plugin_exit` | plugin | The plugin exited with a non-zero code. The code is in `details.exitCode`. |
| `plugin_timeout` | plugin | The plugin ran past its timeout, and the CLI stopped it. |
| `plugin_interrupted` | plugin | The run was stopped by Ctrl-C, `SIGTERM` or `SIGHUP`. |
| `plugin_start_failed` | plugin | The CLI could not start the plugin process. |

Some errors have no reason of their own. They carry the reason `unclassified` with the code `unknown`, and the message names the cause. The table for running a plugin in [Troubleshooting](troubleshooting.md#running-a-plugin) lists the refusal caused by `AUTH0_CLI_PLUGINS_DISABLED`. The other messages without a reason are listed in [Install and update](troubleshooting.md#install-and-update) and in [Other messages](troubleshooting.md#other-messages).

## Proxy error codes

The proxy answers a call that it does not forward with the HTTP status below. The body has `statusCode`, `error`, `message` and `errorCode`. Read `errorCode` to find the reason.

| `errorCode` | HTTP status | When |
| --- | --- | --- |
| `invalid_proxy_token` | 401 | The per-run secret is missing or wrong. |
| `bad_request` | 400 | The request is malformed, its path is not canonical or ambiguous, or its query string has characters that are not allowed. |
| `method_not_allowed` | 405 | The HTTP method is not allowed for this path. |
| `uri_too_long` | 414 | The request target is too long. |
| `forbidden_path` | 403 | The path is not allowed through the proxy. |
| `no_scope_rule` | 403 | The endpoint is not in the scope table of the registry index. For a linked build, no rule of its dev policy matches the call. |
| `insufficient_scope` | 403 | The command did not declare a scope that allows the call. The message names the scope to add. |
| `payload_too_large` | 413 | The request body is too large. |
| `session_expired` | 401 | The session expired or was rejected, and the proxy cannot renew it. For a user login, run `auth0 login`. |
| `unsupported_response` | 502 | The response from Auth0 is not JSON, or it is too large for the proxy to check. |
| `upstream_redirect` | 502 | Auth0 answered with a redirect, which the proxy does not follow. |
| `upstream_failed` | 502 | The proxy could not reach Auth0, or could not read its response. |

The proxy renews a token of a client-credentials session when Auth0 answers 401. It does this up to three times in one run.

## Next

- [Building a plugin](building.md) walks through a project from the first command to the approval.
- [Releasing a plugin](releasing.md) explains how a version becomes a release.
- [Security and trust model](security.md) explains how the CLI checks what it installs and runs.

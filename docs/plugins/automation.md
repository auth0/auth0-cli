---
layout: default
title: Agent mode and automation
parent: Plugins
nav_order: 7
---

This page explains how plugins behave when no person is at the keyboard, in agent mode, with `--no-input`, and in CI. You will see what an agent or a script can do, what it is refused, and how to approve a plugin in CI without a prompt.

## The rule

Only a person approves a plugin. Everything else can be automated. An agent or a script can search, inspect, install, update and run plugins, but it cannot record an approval. Two ways to skip the prompt in CI: approve in a terminal first, or pin the hash in an environment variable. The pin is described below.

## Agent mode

Agent mode turns on JSON output, turns off prompts and turns off colors. It is on when you pass `--agent-mode`, when `AUTH0_AGENT_MODE=true` is set, or when the CLI detects an AI agent. Pass `--agent-mode=false` or set `AUTH0_AGENT_MODE=false` to turn it off.

Errors in agent mode, and with `--json`, are one JSON object on stderr. This is the error when `plugin approve` runs without a tenant to approve for:

```json
{"error":{"code":"validation","reason":"plugin_no_tenant","message":"there is no tenant to approve the plugin for. Pass --tenant <tenant>, or log in with: auth0 login"}}
```

`code` is the class of the failure, and `reason` is the specific cause. Scripts should branch on `reason`, because the message can change between releases. `details` is added when there is more to say. The reason codes for plugins are listed in [Troubleshooting](troubleshooting.md) and [Reference](reference.md).

### Install and update

In agent mode, with `--no-input`, and when there is no terminal, `auth0 plugin install` installs the plugin without an approval and exits with code 0. The output says what is still missing:

```bash
auth0 plugin install checkmate --tenant acme.us.auth0.com --agent-mode
```

```json
{
    "id": "auth0/checkmate",
    "version": "1.0.0",
    "previousVersion": "",
    "action": "installed",
    "pinned": false,
    "approvalRequired": true,
    "approveCommand": "auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"
}
```

An agent reads `approvalRequired` and passes `approveCommand` to a person. No flag approves a plugin during install. Running the plugin stays blocked until a person approves it or a pin covers it.

`auth0 plugin update` is stricter, because new permissions need an approval before they apply. If the new version asks for different permissions and nobody can answer, the update fails. The installed version stays active, and the message says why:

```text
checkmate 1.1.0 asks for permissions that are not approved, and an agent cannot approve plugins. checkmate was not updated, and 1.0.0 stays active. Run this in a terminal to review them: auth0 plugin update checkmate
```

`update --all` reports each plugin as an entry of a JSON array, and it fails if any plugin could not be updated. Pinned plugins and plugins that kept their permissions do not count as failures.

### Approve and uninstall

`auth0 plugin approve` needs a terminal. In agent mode, with `--no-input` and in a script, it exits with code 1 and returns the command that a person has to run as a JSON error:

```json
{"error":{"code":"unknown","reason":"unclassified","message":"only a person can approve a plugin. Run this in a terminal: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"}}
```

`auth0 plugin uninstall` in agent mode requires `--force`, like other destructive commands. Without it, the command fails and removes nothing.

### Running a plugin

A plugin command can run only in the ways that its manifest declares. The CLI refuses any other call before the plugin starts:

| You asked for | Refused when | Reason |
| --- | --- | --- |
| Agent mode | The command does not declare `output.agentMode` | `plugin_output_unsupported` |
| `--json` or `--json-compact` | The command does not declare `output.json` | `plugin_output_unsupported` |
| `--csv` | The command does not declare `output.csv` | `plugin_output_unsupported` |
| Agent mode, `--no-input`, or no terminal | The command is interactive, so it needs a terminal | `plugin_interactive` |

For example, a command that does not declare agent mode fails like this:

```json
{"error":{"code":"usage","reason":"plugin_output_unsupported","message":"auth0 checkmate env does not support agent mode (turn it off with --agent-mode=false), because its manifest does not declare output.agentMode"}}
```

When a plugin starts and then fails, the error has the code `plugin`, and `details` names the plugin and its version. If the plugin exited by itself, `details.exitCode` holds its exit code, and the reason is `plugin_exit`. A plugin that runs past its timeout is stopped along with its child processes, and it fails with `plugin_timeout`:

```json
{"error":{"code":"plugin","reason":"plugin_timeout","message":"checkmate 1.0.0 did not finish within 5s and was stopped","details":{"pluginId":"auth0/checkmate","version":"1.0.0"}}}
```

In agent mode the CLI does not print the line about denied API calls. A failed run carries that line in `details.deniedCalls` instead.

If you type the name of a plugin that is in the registry but not installed, the CLI returns the install command as data:

```json
{"error":{"code":"not_found","reason":"plugin_not_installed","message":"Plugin \"checkmate\" is available. Install it with: auth0 plugin install checkmate","details":{"pluginId":"auth0/checkmate","installCommand":"auth0 plugin install checkmate"}}}
```

### Finding plugin commands

`auth0 commands --json` lists plugin commands next to the core ones. A plugin command has `"source": "plugin"`, its `pluginId` and `pluginVersion`, the `requiredScopes` of the command, and an `output` object. The `output` object says which of `json`, `csv` and `agentMode` the command supports. An agent can pick a command and check which flags it may use without parsing the help text.

## Exit codes

The exit code is `0` on success and `1` on any failure, so scripts that only test for zero keep working. The one exception is an interrupt. When you stop the CLI with Ctrl-C, it exits with `130`. The kind of failure is in the error object, not in the exit code. When a plugin fails, its own exit code is in `details.exitCode`, and the CLI still exits with `1`.

## Approve a plugin in CI

Set `AUTH0_CLI_PLUGIN_APPROVALS` to a comma-separated list of `registry/name@sha256:<hash>` entries. A plugin whose permissions hash matches one of the pins counts as approved on every tenant, so pin only a version you have reviewed. Installing a plugin never approves it. Approval needs a person at a terminal or a pin, as in this example:

```bash
export AUTH0_CLI_PLUGIN_APPROVALS="auth0/checkmate@sha256:9b991b1b3d0c523b4d7aa0f05b5e970f3621bae92d157d563a0458ba28a5b9a7"
```

Take the hash from the `PERMISSIONS HASH` row of `auth0 plugin info checkmate`, or from the `permissionsHash` key of `auth0 plugin info checkmate --json`. Review the permissions once in a terminal before you pin the hash, because a pin says you reviewed exactly those permissions.

A few rules apply:

- A pin is checked by `install`, `update` and every run. With a matching pin, `install` and `update` do not ask, and the result has `"approvalRequired": false`.
- A pin matches the hash and nothing else. If a new version of the plugin asks for different permissions, the hash changes, and the pin no longer matches. The run then fails with `plugin_not_approved` until you review the new permissions and update the pin.
- For a plugin linked in dev mode, `info` shows a hash that also covers the exact bytes of `dev-policy.json`, and a pin must use that hash. Editing the file changes the hash, so the pin no longer matches.
- You can list more than one pin, for example to cover a rollout: `auth0/checkmate@sha256:<hash>,auth0/checkmate@sha256:<other hash>`. Each entry needs the full ID with the registry name, as in `auth0/checkmate`.
- A pin is never written to `approvals.json`.
- An invalid value is an error, not an empty list, so a typo cannot look like having no pins:

```text
The value of AUTH0_CLI_PLUGIN_APPROVALS is not valid. (the entry "checkmate@sha256:abc" is not in the form registry/name@sha256:<64 lowercase hex digits>)
```

A pin works with `AUTH0_CLI_AUTH_MODE=env`, where the CLI reads the tenant and the credentials from the environment:

```bash
export AUTH0_CLI_AUTH_MODE=env
export AUTH0_DOMAIN=acme.us.auth0.com
export AUTH0_CLIENT_ID=<client-id>
export AUTH0_CLIENT_SECRET=<client-secret>
auth0 plugin install checkmate@1.0.0
auth0 checkmate --json
```

In this mode the CLI does not save the tenant or the credentials in `config.json` or in the keychain. Plugin files still go to the plugins folder described in [Managing plugins](managing.md).

In this mode the tenant is fixed by `AUTH0_DOMAIN`. Passing a different `--tenant` fails instead of running on another tenant:

```text
--tenant "other.us.auth0.com" conflicts with AUTH0_DOMAIN="acme.us.auth0.com"; in AUTH0_CLI_AUTH_MODE=env mode the tenant is fixed by AUTH0_DOMAIN. Drop --tenant, or set AUTH0_DOMAIN to the tenant you want to target.
```

An exact version install pins the plugin. So `auth0 plugin install checkmate@1.0.0` also stops `update --all` from moving it in a long-lived environment.

## Turn plugins off

`AUTH0_CLI_PLUGINS_DISABLED=true` blocks install, update, approve and running plugins. Search, list, info and uninstall keep working. Use it on managed machines and in images where plugins are not wanted:

```text
 ▸    Plugins are turned off on this machine by AUTH0_CLI_PLUGINS_DISABLED=true, so auth0 plugin install is not available. Searching, listing and uninstalling still work.
```

## Other switches

| Variable | Effect |
| --- | --- |
| `AUTH0_CLI_PLUGIN_UPDATE_HINT=false` | Turns off the hint about a newer plugin version, which shows at most once a day. The hint never shows in agent mode, with `--no-input`, with `--json`, `--json-compact` or `--csv`, or when the output is not a terminal. |
| `AUTH0_CLI_ANALYTICS=false` | Turns off usage events, including the plugin events. See [Security and trust model](security.md). |

## Next

- [Troubleshooting](troubleshooting.md): reason codes and fixes.
- [Reference](reference.md): environment variables and reason codes in one place.
- [Managing plugins](managing.md): the commands behind these flows.

---
layout: default
title: Troubleshooting
parent: Plugins
nav_order: 8
---

This page lists the errors you can get from the plugin commands, what each one means, and what to do about it. Find the message you see, or the `reason` from the JSON error, and follow the fix.

## Read an error

In a terminal, the CLI prints the message under a heading. With `--json`, `--json-compact` or in agent mode, the error is one JSON object on stderr. The `reason` is the stable part, and the message can change between releases:

```json
{"error":{"code":"validation","reason":"plugin_registry_unavailable","message":"..."}}
```

The exit code is `1` for every failure, except an interrupt, which exits with `130`. See [Agent mode and automation](automation.md) for the error object and its `details`.

Some errors have no reason. They show `"code":"unknown"` and `"reason":"unclassified"`, and the message names the cause. Those errors are listed under [Install and update](#install-and-update) and [Other messages](#other-messages).

Each one names the file, plugin, version or tenant involved, and the next command to run.

## Running a plugin

When you run `auth0 <plugin>`, the CLI checks things in a fixed order and stops at the first failure. The order is described in [Getting started](getting-started.md). This table follows it.

| Reason | What it means | What to do |
| --- | --- | --- |
| No reason. The message names `AUTH0_CLI_PLUGINS_DISABLED`. | Plugins are turned off on this machine. | Unset the variable, or set it to `false`. Any value other than `true` or `false` also turns plugins off. |
| `plugin_verification_failed` | A file of the installed plugin no longer matches what was checked at install time, so the plugin does not run. | Reinstall it with `auth0 plugin install <name>`. |
| `plugin_registry_unavailable` | The registry index is missing, expired, or older than the 30 days a plugin may run on a saved copy. | Reconnect, then run `auth0 plugin search` to refresh the index. |
| `plugin_revoked` | The registry revoked this version. | Run `auth0 plugin update <name>`. |
| `plugin_interactive` | The command needs a terminal, and you used agent mode, `--no-input`, or no terminal. | Run it in a terminal. |
| `plugin_output_unsupported` | You asked for `--json`, `--json-compact`, `--csv` or agent mode, and the command does not declare it. | Remove the flag. For agent mode, use `--agent-mode=false`. |
| `plugin_not_approved` | You have not approved the plugin for this tenant, or it asks for something different from what you approved. | Run the `auth0 plugin approve` command in the message. |
| `plugin_approvals_damaged` | `approvals.json` cannot be read. | Remove the file named in the message, then approve again. The CLI does not delete it on its own. |
| `plugin_approval_check` | The approval check failed for another reason. The message says why. | Read the message and fix the value it names. |
| `plugin_no_tenant` | The CLI does not know which tenant to use. | Pass `--tenant <tenant>`, or log in with `auth0 login`. |
| `plugin_session_unavailable` | There is no usable session for the tenant. | Run `auth0 login`. |
| `plugin_tenant_mismatch` | The session is for a different tenant than the one the plugin is approved for. | Pass `--tenant` for the approved tenant, or approve the plugin for the tenant you are logged in to. |
| `plugin_timeout_invalid` | The timeout in the plugin's manifest is not a usable duration. | Tell the plugin's owner. `auth0 plugin info` shows who that is. |
| `plugin_command_missing` | The plugin has no command with that name any more. | Run it again. If it persists, reinstall the plugin. |
| `plugin_changed` | The plugin changed on disk while the CLI was opening the session. | Run the command again. |
| `plugin_ambiguous_subcommand` | A call of the root command has a subcommand name among its arguments, before a bare `--`, so the CLI cannot tell which command you meant. | Type the subcommand first, write the flag as `--flag=value`, or put the argument after `--`. |

### Not approved

```text
=== other.us.auth0.com error

 ▸    checkmate is not approved for other.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant other.us.auth0.com
```

Approval belongs to a tenant and to a set of permissions. A new version that asks for something different gives the same error with different text:

```text
 ▸    checkmate changed what it asks for since you approved it for acme.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com
```

### A plugin that does not verify

If the executable was changed after the install, the run stops:

```text
 ▸    checkmate no longer matches what was verified when it was installed (the hash of checkmate changed), so it will not run. Reinstall it with: auth0 plugin install checkmate
```

If `plugin.json` was changed, the plugin is not offered as a command, and running `auth0 checkmate` does not run it. When the saved registry index lists the plugin, the CLI says that it is available to install:

```text
 ▸    Plugin "checkmate" is available. Install it with: auth0 plugin install checkmate
```

If the saved index does not list it, the CLI reports the command as unknown. In both cases, `auth0 plugin info checkmate` shows `INSTALLED  1.0.0 (failed verification)` and says what changed. The fix is the install command from the message.

### The registry index is old or cannot be reached

Searching and running a plugin use the saved copy of the registry index when the registry cannot be reached. The CLI tells you how old the copy is:

```text
 ▸    The registry could not be reached at <URL of the file>: <error> (using the cached index from 4 days ago)
```

The CLI uses the saved copy in the same way when the registry serves older metadata or an older index than the CLI has already accepted, and when another auth0 process is refreshing the registry. When the registry has stopped re-signing its metadata, so that the timestamp it serves has expired, the warning reads:

```text
 ▸    The registry has not been re-signed since 2026-10-01T00:00:00Z (using the cached index from 9 days ago)
```

In all of these cases the time of the last refresh does not change, so the saved copy keeps its age. A plugin keeps running on the saved copy for up to 30 days after the last good refresh. After that, it stops with `plugin_registry_unavailable`:

```text
 ▸    checkmate 1.0.0 was not run, because the registry index cannot be trusted: The registry index was last refreshed 44 days ago, so installed plugins cannot run until it is refreshed. Reconnect and run: auth0 plugin search.
```

If you are behind a proxy, set `HTTPS_PROXY`. If the registry address is blocked, point the `auth0` registry at a mirror, as described in [Managing plugins](managing.md).

## Install and update

Installing and updating need an index that was refreshed within the last day. When the refresh fails, the message looks like this, with the command you ran in place of the placeholder:

```text
 ▸    installing a plugin needs a current registry index, but the registry at <registry URL> could not be refreshed: <why>. Reconnect and run: auth0 plugin install <plugin>
```

These errors have a reason:

| Reason | What it means | What to do |
| --- | --- | --- |
| `plugin_reference` | The plugin name or version is not valid. | Names use lowercase letters, digits and hyphens, and start with a letter or digit. A version has the form `MAJOR.MINOR.PATCH`. To install an exact version, use `auth0 plugin install name@version`. |
| `plugin_name_reserved` | The plugin has the name of a core command. | The plugin cannot be installed under that name. The message says `auth0 already has a command named "<name>". Plugin names cannot shadow core commands`. |
| `plugin_not_installed` | `update`, `uninstall` or `approve` named a plugin that is not installed. | Install it first, or check the spelling. The message lists the installed plugins, or says that none are installed. |
| `plugin_update_target` | `plugin update` was run with no name and without `--all`. | Give one name, or use `--all`. |
| `plugin_no_tenant` | `approve` has no tenant to approve the plugin for. | Pass `--tenant <tenant>`, or log in with `auth0 login`. |

Install works without a tenant. It installs the plugin and tells you to approve it later. An update that asks for new permissions and has no tenant to ask about keeps the installed version, and it says so.

Other errors from these commands have no reason, so the JSON shows `unclassified`. The message names the cause. The table below lists the messages you are most likely to see. The plugin name is the short name, such as `checkmate`, or the full ID, such as `auth0/checkmate`, depending on the message.

| Message starts with or contains | What it means | What to do |
| --- | --- | --- |
| `<name> is not listed in the auth0 registry.` | There is no such plugin in the registry. | Check the spelling with `auth0 plugin search`. |
| `<name> <version> is not available in the auth0 registry.` | The version does not exist. The message lists the versions that do. | Pick one of them. |
| `<name> <version> has no build for <machine>.` | The version has no build for your operating system and architecture. The message lists the platforms it supports. | Install one of those versions, or use another machine. |
| `<name> <version> requires auth0-cli <range>. You have <version>.` | The plugin needs another version of the CLI. | Update the CLI. |
| `<name> <version> targets plugin API version <n>, which needs a newer auth0 CLI.` | The plugin was built for a newer plugin API. | Update the CLI. |
| `<name> <version> was revoked: <reason>.` | The registry revoked this version. | Install a newer one with `auth0 plugin update <name>`. |
| `Downloaded file does not match the registry hash. The file was discarded.` | The download is not the file the signed registry lists. Nothing was installed. | Try again. If it repeats, do not install the plugin, and report it as described in [Security and trust model](security.md). |
| `Downloaded file does not have the size the registry lists.` or `The download is larger than allowed.` | The file is not the one the registry lists, for the same reason. | Try again. If it repeats, report it. |
| `The plugin could not be downloaded. The registry lists it, but the file is unavailable.` | The release file is missing, or the server refused it. | Try again later. If it persists, tell the plugin's owner. |
| `Plugins can only be downloaded over https from GitHub release hosts.` | The registry points at a host that plugins cannot come from. | Report it. This is a registry problem. |
| `The plugin archive is not a valid plugin archive.` or `The artifact does not match the registry record.` | The file is not what the registry describes. Nothing was installed. | Report it to the plugin's owner. |
| `The auth0 registry index failed signature verification.` | The signed registry metadata does not have enough valid signatures from the keys the CLI trusts, or the index does not have the length and hash that the signed metadata gives. The same message appears when a saved root does not verify. The CLI never falls back to an unverified index, and it does not use the saved copy after this error. | Check for a proxy or mirror that changes the files. If there is none, report it. If the message names a saved file and a folder to remove, remove that folder to start over. |
| `The registry index expired` | The root, the snapshot or targets file, or the index itself is past its expiry date. The message names the file. An expired timestamp alone does not cause this error when a saved copy is usable, because the CLI shows the "has not been re-signed" warning instead. | Reconnect and refresh with `auth0 plugin search`. If it persists, the registry has not been re-signed, so report it. |
| `The registry index is older than one that was already accepted.` | The registry served an older index, or older metadata, than one the CLI already saw, or its files are out of step with each other. When a saved copy is usable, the CLI uses it and shows this as a warning. | Try again later. If it persists, report it. |
| `The registry index uses a newer format. Update the auth0 CLI to use it.` | The CLI is too old for the registry. | Update the CLI. |
| `Another auth0 process is changing the installed plugins. Try again in a moment.` | Two CLI processes tried to change the plugins at once. | Run the command again. |
| `Another auth0 process is refreshing the registry. Try again in a moment.` | Two CLI processes tried to refresh the registry index at once. | Run the command again. |
| `Another auth0 process changed this plugin while the change was being approved. Run the command again.` | The plugin changed after you answered the consent screen. Nothing was changed. | Run the command again. |

### Other messages

Some messages come from the approvals and the lockfile. They have no reason, so the JSON shows `unclassified`:

| Message starts with or contains | What it means | What to do |
| --- | --- | --- |
| `The plugin approvals file is damaged.` | `approvals.json` cannot be read, so the CLI does not treat it as empty. The message names the file. | Remove the file that the message names, then approve the plugin again. |
| `The plugin lockfile is damaged.` | `plugins.lock.json` cannot be read. The CLI never replaces it on its own. | Remove the `plugins` folder described in [Managing plugins](managing.md), then install again. |
| `only a person can approve a plugin.` | `approve` ran where nobody can answer, such as agent mode or `--no-input`. | Run the command shown in the message in a terminal. |
| `this is a destructive command; re-run with --force` | `uninstall` ran in agent mode without `--force`. | Add `--force`, or run the command in a terminal. |

An invalid `AUTH0_CLI_PLUGIN_APPROVALS` value is also an error. Its reason is `local_validation`. The message is `The value of AUTH0_CLI_PLUGIN_APPROVALS is not valid.` followed by the entry that is wrong, and the form it must have. Each entry is `registry/name@sha256:<hash>`, separated by commas. See [Agent mode and automation](automation.md).

If the pin is valid, but the run still says the plugin is not approved, the plugin asks for different permissions than the ones you pinned. Read the current hash with `auth0 plugin info <name>`, and compare it with the pin.

## The plugin ran and failed

When the plugin starts and then fails, the error has the code `plugin`. The reason says how it ended:

| Reason | What it means | What to do |
| --- | --- | --- |
| `plugin_exit` | The plugin exited with a code other than zero. The message shows the code, and agents get it as `details.exitCode`. | Read what the plugin printed. The plugin's owner explains its exit codes. |
| `plugin_timeout` | The plugin ran longer than its timeout, and the CLI stopped it along with its child processes. | Run it again. If the task is long, ask the plugin's owner about the timeout. |
| `plugin_interrupted` | You interrupted the run, for example with Ctrl-C. | None. |
| `plugin_start_failed` | The CLI could not start the plugin. The message has the cause. | Reinstall the plugin. If it persists, report it with the full message. |

If the CLI printed that the proxy denied API calls, the plugin asked for something that its manifest does not declare:

```text
 ▸    Plugin auth0/checkmate: 2 API calls were denied (read:users for GET /api/v2/users, GET /oauth/token). Review permissions.scopes in plugin.json.
```

The plugin may still have finished. The line is written for the plugin's author, so pass it to the owner listed as `OWNER` in `auth0 plugin info`. The line can also say that Auth0 refused calls because your session lacks a scope. In that case, run the `auth0 login` command that the install warning gave you, with the scopes it lists.

## Author commands

These reasons come from the commands under `auth0 plugin dev`. All of them need `AUTH0_CLI_PLUGIN_DEV_MODE=true`. See [Building a plugin](building.md) and [Reference](reference.md).

| Reason | What it means |
| --- | --- |
| `plugin_dev_mode_required` | The author commands are off. Set `AUTH0_CLI_PLUGIN_DEV_MODE=true`. |
| `plugin_create_invalid` | `plugin dev create` was given a value it cannot use. The message names it. |
| `plugin_link_invalid` | A link cannot be made, or the linked build cannot be used. The message says why, and names the `validate` and `unlink` commands. |
| `plugin_not_linked` | `plugin dev unlink` named a plugin that is not linked. The message lists the linked plugins, or says that none are linked. |
| `plugin_manifest_invalid` | `plugin dev validate` found problems in `plugin.json`. The `details` object has a `problems` list, and each entry has the `path` of the field and a `message`. With `--json`, the command also prints the same list on stdout, next to `valid`. |

## Start over

If the plugin state looks wrong and nothing above fits, `auth0 plugin uninstall <name>` removes one plugin with its versions and approvals. To clear everything, remove the `plugins` folder described in [Managing plugins](managing.md). The next install downloads the registry index again and asks for approval again. Nothing else in the CLI's configuration lives in that folder.

## Report a problem

If you think a plugin or the registry is unsafe, or a check failed in a way you cannot explain, follow the reporting steps in [Security and trust model](security.md). For a bug in the plugin commands, open an issue on the CLI repository. Include the full message, the reason code, the plugin and its version, and the version of the CLI. Remove any tokens and client secrets before you paste output.

## Next

- [Reference](reference.md): reason codes, environment variables and paths in one place.
- [Agent mode and automation](automation.md): the error object and exit codes.
- [Managing plugins](managing.md): where the files are and how to remove them.

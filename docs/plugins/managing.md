---
layout: default
title: Managing plugins
parent: Plugins
nav_order: 2
---

This page covers what you do with a plugin after the first install. You can list and inspect the plugins you have, pin a version or go back to an older one, update, approve a tenant, uninstall, and find the files on disk.

## List what is installed

```bash
auth0 plugin list
```

```text
  ID                VERSION  PINNED  PREVIOUS  SIZE
  auth0/checkmate   1.1.0    no      1.0.0     71.2 MB
```

| Column | Meaning |
| --- | --- |
| `VERSION` | The version that runs. |
| `PINNED` | `yes` when you installed an exact version, so `update --all` leaves the plugin alone. |
| `PREVIOUS` | The version that was active before. It stays on disk, so you can go back without a download. |
| `SIZE` | The space the plugin uses on disk. |

If you linked a plugin from a local build in dev mode (the mode that turns on the author commands, see [Building a plugin](building.md)), a `LINKED` column appears. See [Building a plugin](building.md).

The command also takes `--json` and `--csv`. The JSON entries have the keys `id`, `name`, `version`, `pinned`, `previous`, `sizeBytes` and `linked`.

## Look at a plugin

`auth0 plugin info` shows one version of a plugin, whether it is installed, and the tenants where you approved it:

```bash
auth0 plugin info checkmate
auth0 plugin info checkmate@1.0.0
```

```text
  ID                auth0/checkmate
  VERSION           1.1.0
  DESCRIPTION       Run a tenant security audit
  OWNER             CheckMate team https://example.com/checkmate/bugs
  REQUIRES          none
  PLATFORMS         darwin-arm64, darwin-x64, linux-x64, linux-arm64, linux-x64-musl, linux-arm64-musl, windows-x64
  PERMISSIONS       5 scopes, 1 host, 1 file path, 1 environment variable
  PERMISSIONS HASH  sha256:9b991b1b3d0c523b4d7aa0f05b5e970f3621bae92d157d563a0458ba28a5b9a7
  VERSIONS          1.1.0 (latest), 1.0.0
  INSTALLED         1.1.0
  APPROVED TENANTS  acme.us.auth0.com
```

A few values need explaining:

- `REQUIRES` is the range of CLI versions that the plugin works with. It reads `auth0-cli >=1.38.0` when the plugin sets one.
- `VERSIONS` marks `(latest)` and `(revoked)`. A revoked version cannot be installed or run.
- `INSTALLED` adds `(failed verification)` when the files on disk no longer match what was checked at install time. The message that comes with it tells you to reinstall.
- `APPROVED TENANTS` adds `(different permissions)` to a tenant where you approved an older set of permissions than the ones shown.
- A `HOMEPAGE` row appears when the plugin sets one.
- A `DEPRECATED` row appears when the registry marks the version as deprecated. The row shows the message, and `(use <replacement> instead)` when there is a replacement. A deprecated plugin keeps working.
- For a plugin that you linked from a local build in dev mode, `info` describes the manifest of that build and adds a `LINKED` row.

The `PERMISSIONS HASH` is the value that you pin in CI. See [Agent mode and automation](automation.md).

`auth0 plugin info checkmate --json` returns the same fields for scripts, including `permissionsHash`.

## Install an exact version, or go back

Install an exact version with `name@version`:

```bash
auth0 plugin install checkmate@1.0.0
```

An exact version is pinned. `auth0 plugin update --all` skips pinned plugins and tells you which newer version it left out. The install also tells you how to move on:

```text
 ▸    auth0/checkmate 1.0.0 is already installed.
 ▸    auth0/checkmate is pinned to 1.0.0, so `auth0 plugin update --all` leaves it alone. Move it to the latest version with: auth0 plugin update checkmate
```

The same command is how you go back. The CLI keeps the active version and the one before it on disk. Installing the previous version again only switches to it, without a download:

```text
 ▸    Switched auth0/checkmate from 1.1.0 to 1.0.0.
 ▸    auth0/checkmate is pinned to 1.0.0, so `auth0 plugin update --all` leaves it alone. Move it to the latest version with: auth0 plugin update checkmate
```

A version older than the previous one is downloaded again. If the version that you switch to asks for different permissions than the active one, a terminal asks you to approve them first, as for an update. A version that the registry has revoked is refused, and the message gives the reason.

`name@version` takes `MAJOR.MINOR.PATCH` only. A pre-release version such as `1.0.0-beta.1` is not accepted.

## Update

```bash
auth0 plugin update checkmate
auth0 plugin update --all
```

`update checkmate` moves the plugin to the latest version and clears its pin. `update --all` updates every plugin that is not pinned, and prints one line for each plugin:

```text
 ▸    auth0/checkmate is pinned to 1.0.0 and was skipped. Version 1.1.0 is available: auth0 plugin update checkmate
 ▸    auth0/other 2.0.0 is up to date.
```

The new version is downloaded and verified before anything changes, so an update either completes or leaves the old version active. When the new version asks for different permissions, the CLI asks you to approve them for the current tenant. It shows only what changed when you approved the installed version on that tenant. The old version stays active if you decline, if there is no terminal, in `--no-input` mode, or in agent mode.

Declining at a terminal is not a failure, so the command exits with code 0. With no terminal, in `--no-input` mode or in agent mode, a named update fails. `--all` fails when some plugin could not be updated. Pinned plugins and plugins you declined do not count as failures.

Approval is per tenant, so other tenants are asked at their next run. When the tenant you are on has not approved the result, install and update print a line like this one, with the command that approves it:

```text
 ▸    auth0/checkmate is not approved for acme.us.auth0.com yet. Approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com
```

With `--json`, `update` prints one entry for each plugin, with the keys `id`, `version`, `previousVersion`, `action`, `pinned` and `approvalRequired`. When they apply, it also prints `approveCommand`, `available` and `error`. The `action` is one of `installed`, `switched`, `unchanged`, `skipped`, `declined` or `failed`. A new version that was downloaded is reported as `installed`.

Installing and updating need a current copy of the registry index. They fail when the registry cannot be reached and the saved copy is more than a day old. Searching and running a plugin fall back to the saved copy, and a plugin keeps running from it for up to 30 days. See [Troubleshooting](troubleshooting.md).

## Approve a tenant

A plugin runs on a tenant only after you approved its permissions there. Install asks about the tenant you are on. For another tenant, use `approve`:

```bash
auth0 plugin approve checkmate --tenant acme.us.auth0.com
```

This needs a terminal. In agent mode, with `--no-input`, or in a script, it fails and prints the command to run, because only a person approves a plugin. `approve` always shows the full consent screen, even when you approved an older version. It also works for a plugin that you linked from a local build in dev mode. Without `--tenant` and without a default tenant, it fails with the reason `plugin_no_tenant`.

```text
 ▸    Approved auth0/checkmate 1.0.0 for acme.us.auth0.com.
```

Approval is keyed to the permissions, not the version. A new version that asks for the same permissions needs no new approval. A version that asks for anything else stops with this message until you approve again:

```text
 ▸    checkmate changed what it asks for since you approved it for acme.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com
```

A tenant keeps one approval for each plugin, so approving new permissions replaces the earlier ones. Logging out of a tenant with `auth0 logout` deletes that tenant's approvals, so an approval never outlives the login it was given in. Logging in again means approving again.

## Uninstall

```bash
auth0 plugin uninstall checkmate
```

In a terminal, the CLI asks first:

```text
Are you sure you want to uninstall auth0/checkmate? This removes every version and its approvals. (y/N)
```

If you confirm, the CLI removes every version, the lockfile entry and the approvals on every tenant. It then tells you how much space it freed:

```text
 ▸    Uninstalled auth0/checkmate and freed 71.2 MB.
```

Answer anything but `y` and nothing is removed. Use `--force` to skip the question. Without a terminal, or with `--no-input`, uninstall does not ask. In agent mode it refuses to run without `--force`, and it says `this is a destructive command; re-run with --force to proceed without a confirmation prompt`. `auth0 plugin remove` is an alias. Uninstall works without the registry and when plugins are turned off. It also removes an `approvals.json` file that cannot be read.

## Where the files are

All plugin state is in a `plugins` folder next to the CLI's `config.json`. The CLI builds that path from the `HOME` environment variable on every operating system. On macOS and Linux that is `~/.config/auth0/plugins`. On Windows it is the same relative path, under the folder that `HOME` names. `XDG_CONFIG_HOME` is not used.

```text
plugins/
  plugins.lock.json                installed plugins: versions, hashes, active and previous version
  .lock                            the lock for the plugins and the lockfile
  .approvals.lock                  the lock for approvals.json
  .dev-links.lock                  the lock for dev-links.json
  approvals.json                   approvals for each tenant, keyed by the permissions hash
  hint.json                        when the update hint was last shown
  dev-links.json                   plugins linked in dev mode, only if you link one
  registries/auth0/metadata/       the latest signed root, timestamp, snapshot and targets files
  registries/auth0/roots/          the roots after the first one, such as 2.root.json, only after a key rotation
  registries/auth0/index.json      the last verified registry index
  registries/auth0/state.json      the newest index version accepted, and when the index was last refreshed
  registries/auth0/.lock           the lock for refreshing the index
  store/auth0/checkmate/1.1.0/     the plugin: the executable checkmate (checkmate.exe on Windows) and plugin.json
  store/auth0/checkmate/1.0.0/     the previous version
  tmp/                             downloads in progress
```

The folders, the lockfile, the approvals and the manifests are readable only by your user. The executable is the one file marked as runnable. The active version is a field of the lockfile and not a symbolic link, so switching versions is one file write. Do not edit these files. Before every run, the CLI checks the executable and the manifest against the hashes in the lockfile. A changed file stops the run with the reason `plugin_verification_failed`.

## Turn plugins off

Set `AUTH0_CLI_PLUGINS_DISABLED=true` on machines where you do not want plugins. Install, update, approve and running a plugin then fail with a message that names the variable. Search, list, info and uninstall keep working. Any value that is not `true` or `false` also turns plugins off, so a typo cannot leave them on by accident.

## Registry mirror and proxy

The CLI downloads through the proxy named in `HTTPS_PROXY`. If you cannot reach the registry address, you can point the `auth0` registry at a mirror in `config.json`. The `url` is the base URL of the directory that holds the registry files, not the URL of `index.json`. The CLI adds a slash at the end when it is missing, and it sends a query string on the URL with every request. The metadata and the index are still checked against the root inside the CLI, and plugin files are still downloaded only from GitHub release hosts:

```json
{
  "registries": [
    { "name": "auth0", "url": "https://mirror.example.com/registry/" }
  ]
}
```

A mirror must serve the same files as the registry, under that base URL:

```text
timestamp.json
<N>.root.json
<N>.snapshot.json
<N>.targets.json
targets/<sha256>.index.json
plugins/<name>.json
```

`<N>` is a version number, and `<sha256>` is the hash of the index. The default URL is `https://raw.githubusercontent.com/auth0/auth0-cli-plugins/registry/`. The URL must use `https`. In `AUTH0_CLI_AUTH_MODE=env` the CLI does not read `config.json`, so the mirror does not apply there.

The `auth0` registry is the only one. Every install comes from the signed index, so the CLI does not install from a local file.

## Next

- [Agent mode and automation](automation.md): approvals and updates without a person at the keyboard.
- [Troubleshooting](troubleshooting.md): errors and their reason codes.
- [Security and trust model](security.md): what the checks protect against.

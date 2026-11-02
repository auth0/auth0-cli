---
layout: default
title: Getting started
parent: Plugins
nav_order: 1
---

This page shows you how to find a plugin, look at what it asks for, install it, approve it for your tenant, and run it. The examples use a plugin called `checkmate`. Plugin commands are Experimental, so their output and behavior can change between releases.

The steps on this page show the flow that plugins follow once the registry is open. The search and info output below is sample output. For what works today, see [Status](index.md#status).

## What a plugin is

A plugin is a program that the Auth0 CLI downloads, checks and starts for you. You run it as `auth0 <name>`, and the CLI is the only supported way to start it. The CLI checks your approval and gives the plugin a proxy session, so running the plugin binary directly skips those checks and gives it no Auth0 access.

Searching, inspecting and installing plugins does not need a login. Running a plugin does, because the plugin calls the Management API (the API that reads and changes your tenant) through your session. Log in first with `auth0 login`. For CI and agents, [Agent mode and automation](automation.md) describes an environment variable mode that needs no saved login.

## Find a plugin

List every plugin in the registry:

```bash
auth0 plugin search
```

```text
  ID                VERSION  INSTALLED  DESCRIPTION
  auth0/checkmate   1.0.0    no         Run a tenant security audit
```

Add a word to filter the list. The word is matched against the plugin ID and the description, and the case does not matter:

```bash
auth0 plugin search security
```

Before you install anything, look at what the plugin asks for:

```bash
auth0 plugin info checkmate
```

```text
  ID                auth0/checkmate
  VERSION           1.0.0
  DESCRIPTION       Run a tenant security audit
  OWNER             CheckMate team https://example.com/checkmate/bugs
  REQUIRES          none
  PLATFORMS         darwin-arm64, darwin-x64, linux-x64, linux-arm64, linux-x64-musl, linux-arm64-musl, windows-x64
  PERMISSIONS       5 scopes, 1 host, 1 file path, 1 environment variable
  PERMISSIONS HASH  sha256:9b991b1b3d0c523b4d7aa0f05b5e970f3621bae92d157d563a0458ba28a5b9a7
  VERSIONS          1.0.0 (latest)
  INSTALLED         no
  APPROVED TENANTS  none
```

`OWNER` is the team to contact about the plugin. `PERMISSIONS` counts what the plugin declares, and the install shows you the full list. `PERMISSIONS HASH` is a fingerprint of those permissions, and it changes when they change. You can give a plugin as `checkmate`, as `auth0/checkmate`, or with a version as `checkmate@1.0.0`. The registry is called `auth0`, and it is the only one for now.

## Install a plugin

```bash
auth0 plugin install checkmate
```

The CLI checks the registry, shows you what the plugin asks for on the consent screen (the list of permissions that you approve or decline), and downloads the plugin only after you agree:

```text
 ▸    auth0/checkmate 1.0.0  Run a tenant security audit
 ▸    Download: 71.2 MB (darwin-arm64)

This plugin asks to use your Auth0 tenant acme.us.auth0.com:

  Can read     clients, connections, tenant settings
  Can change   clients (update), connections (update)
  Hosts        api.github.com    Check SDK versions
  Files        $CWD (write)    Save the report
  Environment  GITHUB_TOKEN    Call the GitHub API
  Interactive  no

Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI.

Approve for acme.us.auth0.com? [y/N/d]  (d shows the exact scopes)
```

The last line of the consent screen names the tenant that you pass with `--tenant`, or your default tenant. Type `y` to approve, `d` to see the exact scope names, and anything else to decline. After `d`, the CLI lists the scopes and asks again with `[y/N]`.

If you decline, nothing is downloaded or installed. The command exits with code 0, because declining is not a failure:

```text
 ▸    auth0/checkmate was not approved, so nothing was installed. To review the permissions again, run: auth0 plugin install checkmate
```

If the CLI has no tenant to ask about, because you passed no `--tenant` and have no default tenant, it installs the plugin without asking. It then tells you to log in and approve the plugin later.

Each part of the screen means something different:

| Row | What it means |
| --- | --- |
| Can read, Can change | The Management API scopes the plugin may use. The proxy that the CLI starts for the plugin refuses calls outside the scopes of the command that you ran. A scope that is neither a read nor a change is listed under `Other`. A plugin with no scopes shows `Auth0 API  none`. |
| Hosts | Other hosts the plugin says it contacts, with the reason. |
| Files | Paths the plugin says it reads or writes, with the reason. |
| Environment | Environment variables the plugin asks to receive, with the reason. |
| Interactive | `yes` when at least one command needs a terminal. |

Hosts and files come from the plugin's publisher. The CLI does not enforce them. Scopes and environment variables are enforced. Read the reasons before you approve, and see [Security and trust model](security.md) for how each part is checked.

After you answer `y`, the CLI downloads the file, checks it against the hash in the signed registry index (the list of plugins and versions that the CLI trusts), and installs it. If your login does not have the scopes the plugin asks for, the CLI warns you. The warning gives the `auth0 login` command that adds them. If you logged in with client credentials, the warning says the application needs a grant for those scopes on the Auth0 Management API instead. The CLI never starts a login on its own.

An install that you approve ends with a line like this one. The approval is saved for this tenant only:

```text
 ▸    Installed auth0/checkmate 1.0.0.
```

## Run the plugin

The plugin is now a command of the CLI, and it shows up under its own heading in `auth0 --help`:

```text
Plugin Commands:
  checkmate       Run a tenant security audit
```

Read its help first. The CLI builds the help from the plugin's manifest, so it shows the scopes and the timeout of each command:

```bash
auth0 checkmate --help
```

Then run it:

```bash
auth0 checkmate
```

Before the plugin starts, the CLI runs these checks in order. It stops at the first one that fails:

1. Plugins are not turned off on this machine.
2. The tenant that you asked for matches `AUTH0_DOMAIN`, if that variable fixes the tenant.
3. The installed files match the hashes that were checked at install time.
4. The registry index is recent enough, and the version has not been revoked.
5. The command can run the way you called it. For example, it has a terminal if it needs one, and the output flags you passed are declared in its manifest.
6. You approved the plugin for the tenant.
7. Your session for that tenant is open, and it belongs to the approved tenant.

The plugin never sees your token. The CLI starts a local proxy for the run and gives the plugin its address and a temporary token. When the plugin calls the Management API through the proxy, the proxy swaps in your real token and allows only the scopes that the command declared.

When the plugin finishes, the CLI prints one line if the proxy denied calls. This tells you when a plugin could not get data it asked for, even if the plugin hid the error:

```text
 ▸    Plugin auth0/checkmate: 2 API calls were denied (read:users for GET /api/v2/users, GET /oauth/token). Review permissions.scopes in plugin.json.
```

The second call is outside `/api/v2`, so the proxy refuses it without checking scopes. The last sentence is written for the plugin's author. If you see this line, tell the owner listed as `OWNER` in `auth0 plugin info`.

### Flags and arguments

The CLI reads its own flags wherever you put them: `--tenant`, `--debug`, `--no-input`, `--no-color`, `--agent-mode`, `--json`, `--json-compact`, `--csv` and `--help`. It passes every other argument to the plugin as you typed it, without checking it. Everything after a bare `--` goes to the plugin untouched.

Type the name of a subcommand directly after the plugin name, as in `auth0 checkmate scan --path .`. The CLI refuses a call of the plugin's root command that has a subcommand name among its arguments, before a bare `--`, with the reason `plugin_ambiguous_subcommand`. A flag placed before the subcommand is the usual cause, because the CLI cannot tell the subcommand from the value of that flag.

The output flags work only when the plugin declares them. Asking for `--csv` from a command that does not declare it fails with the reason `plugin_output_unsupported`.

## Approve on another tenant

Approval belongs to a tenant. If you run the plugin on a tenant that you have not approved it for, the run stops before anything else happens:

```text
=== other.us.auth0.com error

 ▸    checkmate is not approved for other.us.auth0.com. Run: auth0 plugin approve auth0/checkmate --tenant other.us.auth0.com
```

Run the command from the message in a terminal. It shows the same consent screen as the install and records your answer:

```bash
auth0 plugin approve checkmate --tenant other.us.auth0.com
```

An approval holds for later versions of the plugin that ask for the same permissions. If a new version asks for anything else, the CLI asks again, as the next section shows.

## Update a plugin

```bash
auth0 plugin update checkmate
```

```text
 ▸    Updated auth0/checkmate from 1.0.0 to 1.1.0.
```

When the new version asks for different permissions, you approve them before the update happens. If you already approved the installed version on this tenant, you see only what changed. If you did not, you see the full consent screen. When the new version asks for the same permissions, the CLI asks nothing. This is an example of the consent screen for a changed version:

```text
auth0/checkmate 1.1.0 asks for different permissions on tenant acme.us.auth0.com
  Added        can read connections
  Removed      can read logs
  Added        host new.example.com    New
  Removed      host old.example.com    Old
  Changed      file $CWD (write)    Save the report
  Changed      variable GITHUB_TOKEN    A new reason
  Added        command fresh (interactive)
  Removed      command gone
  Changed      command toggle is now interactive
Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI.
```

If you decline, the old version stays active and the command says so:

```text
 ▸    auth0/checkmate was not updated, because the new permissions were not approved. 1.0.0 stays active.
```

If there is no terminal to ask in, or you use `--no-input` or agent mode, the old version also stays active. The command then fails and says why. See [Agent mode and automation](automation.md).

Once a day, after a plugin runs, the CLI can print a hint when a newer version is available or when the installed version is deprecated:

```text
 ▸    checkmate 1.1.0 is available. Run: auth0 plugin update checkmate
```

The hint shows only in a terminal, never with `--json`, `--no-input` or agent mode. Set `AUTH0_CLI_PLUGIN_UPDATE_HINT=false` to turn it off.

## Next

- [Managing plugins](managing.md): list, pin, roll back, approve and uninstall.
- [Agent mode and automation](automation.md): use plugins in CI and from agents.
- [Troubleshooting](troubleshooting.md): what the errors mean and how to fix them.

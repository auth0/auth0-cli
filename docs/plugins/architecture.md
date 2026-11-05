---
title: Architecture
parent: Plugins
nav_order: 5
layout: default
---

## What this page covers

This page explains the parts of the plugin system, what happens when you install and run a plugin, and which files the CLI writes on your machine. Use it when you want to know where a plugin's data lives or why a run stopped at a particular check.

## The diagram

In the diagram below, pick a scenario from the dropdown, then use Next, Previous or Play. Each step highlights the component that acts and explains what happens and why. The flows cover install and update with the consent screen, running a command with scope enforcement, creating, validating and linking a plugin, the release template, uninstalling, agent mode and CI.

{% include plugin-flow.html %}

## Components

| Component | What it does |
| --- | --- |
| Auth0 CLI | Registers the plugin commands, runs the checks in order, asks for consent, and starts the plugin. |
| Registry client | Updates the signed registry metadata, checks it against the root built into the CLI, checks the index against it, and keeps the verified copy. |
| Store | Downloads archives from allowed hosts, checks their hashes, unpacks them, writes the lockfile, and checks the files before each run. |
| Approvals | Shows the consent screen, records approvals per tenant, and reads CI pins. |
| Runner | Builds the plugin environment, removes CLI flags from the arguments, and starts and stops the plugin process. |
| Loopback proxy | Serves HTTPS on `127.0.0.1`, an address that only this machine can reach, enforces scopes, strips secret fields, and forwards calls with the real token. |
| Dev links | Records local builds that an author runs with `auth0 plugin dev link`. |
| Scaffold | Writes the starter projects for `auth0 plugin dev create`. |
| Analytics | Sends the plugin usage events, unless analytics are turned off. |
| Plugin | Any language, built by its author. A native executable with a `plugin.json` next to it. It calls the Management API through the proxy. |
| Management API | Receives the calls that the proxy forwards with the real access token. |

The CLI is the only part that talks to the registry, the release hosts and your tenant. A plugin's Management API calls go only through the proxy, but the CLI does not block other network traffic from the plugin. See [Security and trust model](security.md) for that limit.

## Install and update

1. You run `auth0 plugin install <name>`. Install needs no login. It needs a tenant only to record the approval, and it uses `--tenant` or your default tenant.
2. The registry client updates the signed metadata from the registry URL, which is the base URL of the metadata directory. It follows any newer roots from the root built into the CLI, checks the signatures and expiry dates of the timestamp, snapshot and targets files, and downloads `index.json`. It then checks the index against the length and hash in the targets file, its own expiry date, and the newest index it accepted before.
3. The CLI picks the newest version that is not revoked, has an archive for your operating system, CPU and C library, and accepts this CLI version.
4. The consent screen shows the permissions of that version for your tenant, from the signed index. You can answer no, and nothing is downloaded.
5. The store downloads the archive from an allowed host into `tmp/`, stopping if it grows past the size cap. Then it checks the SHA-256 hash against the index.
6. The store unpacks the archive and compares the manifest inside with the index record. Only then are the files moved into `store/`.
7. The lockfile is rewritten through a temporary file and a rename. The approval is recorded last, and only after the install succeeded.

An update follows the same steps. If the permissions hash is the same as the one your tenant approved, the update asks nothing. If it changed, the screen shows only the difference, and the old version stays active until you approve. The previous version stays on disk, so a rollback can reuse it.

## Run

1. You run `auth0 <plugin> <command>`. The CLI takes out the flags it owns, such as `--tenant`, and passes the rest to the plugin untouched.
2. The CLI checks that plugins are not turned off, and that a `--tenant` you passed matches `AUTH0_DOMAIN` when the environment fixes the tenant.
3. The CLI checks the installed files against the lockfile. This happens before any login.
4. The CLI reads the index, checks that the version is not revoked, and checks that the command can run the way it was called.
5. The CLI checks that the tenant approved these permissions. A missing approval stops the run here, before any token is requested.
6. The CLI opens the session for the approved tenant and checks the files a second time.
7. The CLI starts the loopback proxy with the scopes of this command, and a new certificate and secret for the run.
8. The CLI starts the plugin with a short environment that holds the proxy address and the secret. The plugin's output goes straight to your terminal.
9. Each call from the plugin goes to the proxy, which checks the path, the method and the scope, forwards the call with the real token, and removes secret fields from the response.
10. When the plugin exits, the CLI stops the proxy and prints one line about any calls that were denied. That line is not printed in agent mode. The CLI then exits with the result of the run.

For the rules behind each check, see [Security and trust model](security.md).

## Files on disk

Plugin state lives in a `plugins` folder next to `config.json`, in your CLI config directory. The CLI stores no tokens or client secrets there. Those stay in the operating system keyring.

```plaintext
~/.config/auth0/
├── config.json
└── plugins/
    ├── plugins.lock.json          installed plugins, hashes, active and pinned versions (0600)
    ├── approvals.json             approvals per tenant, keyed by permissions hash (0600)
    ├── dev-links.json             local builds from plugin dev link
    ├── hint.json                  when the update hint was last shown
    ├── registries/auth0/
    │   ├── metadata/              latest root, timestamp, snapshot and targets files
    │   ├── roots/                 roots after the first one, as 2.root.json, 3.root.json
    │   ├── index.json             last verified index
    │   ├── state.json             newest index version accepted, last refresh
    │   └── .lock                  held while the registry is refreshed
    ├── store/auth0/checkmate/
    │   └── 1.0.0/
    │       ├── checkmate          the executable (0755), checkmate.exe on Windows
    │       └── plugin.json        the manifest from the archive (0600)
    └── tmp/                       downloads in progress
```

The path above is the one the CLI uses for `config.json`, which is `~/.config/auth0` on macOS and Linux. The lockfile and the approvals file are written to a temporary file and then renamed into place, so an interrupted write does not leave a half-written file behind. The folders are created with mode `0700`.

The first root is built into the CLI, so it is not stored. The `roots` folder holds the roots that the registry published after it, and it only has files once the registry has rotated a key. A refresh writes to these files only after everything verified, so a failed refresh leaves them as they were. When the CLI uses the saved index, it checks the roots, the targets file and the index again without the network.

Deleting the `plugins` folder removes every installed plugin and every approval. It does not remove your login, because the login stays in the keyring and in `config.json`.

## Where to go next

- [Security and trust model](security.md) explains the checks, the limits and how to report a problem.
- [Building a plugin](building.md) walks through `plugin dev create`, `dev link` and `dev validate`.
- [Reference](reference.md) lists the environment variables, reason codes and paths.

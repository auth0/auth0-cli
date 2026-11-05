---
title: Security and trust model
parent: Plugins
nav_order: 6
layout: default
---

## What this page covers

This page explains what a plugin can reach, what the CLI checks before every run, and what it does not protect against. Read it before you install a plugin on a machine that holds production credentials, and before you publish one.

The Auth0 CLI is the only supported way to start a plugin. It checks your approval and gives the plugin a proxy session. Running the plugin binary directly skips those checks and gives it no Auth0 access.

## The short version

- A plugin is a native program that runs as your user on your machine. The CLI controls which Auth0 calls it can make, but it does not run the plugin in a sandbox.
- The CLI never gives a plugin your access token or your client secret. The plugin calls a local proxy, and the proxy adds the token.
- Each command gets only the scopes that the command declares, and the proxy refuses every other call.
- A plugin runs only on a tenant that a person approved, with the permissions that person saw.
- Plugins can still read the files you can read and send data to any host. The CLI does not stop that, so install only plugins you trust.

## What the CLI checks before a run

Every run of `auth0 <plugin>` goes through the same checks, in this order. A check that fails stops the run before the next one starts, and nothing from the plugin runs.

1. Plugins are not turned off on this machine (`AUTH0_CLI_PLUGINS_DISABLED`).
2. When the environment fixes the tenant (`AUTH0_CLI_AUTH_MODE=env`), a `--tenant` that you pass matches `AUTH0_DOMAIN`.
3. The installed executable and `plugin.json` still match the hashes in the lockfile.
4. The registry index is current enough to run from, and it does not revoke this version.
5. The command can run the way it was called. An interactive command needs a terminal, and the output you asked for must be declared.
6. The tenant approved the permissions of this plugin.
7. Only now does the CLI open a session. The session must be for the approved tenant.

The CLI checks the executable and manifest a second time after the session is open, because a login can take a while. If either changed in between, the run stops.

## The registry, signatures and keys

The registry is a signed index that lists each plugin, its versions, the platforms it builds for and the permissions each version asks for. The registry is a TUF repository (The Update Framework), and the index is its one signed file.

- The registry uses four roles: root, targets, snapshot and timestamp. There are no delegations, and the repository uses consistent snapshots, which means every signed file except `timestamp.json` has a version number or a hash in its name.
- The production root has 3 root keys, and 2 of them must sign a new root. The targets, snapshot and timestamp roles each have 1 key, and 1 signature is enough.
- The signed files expire. The root is valid for 365 days, the targets and snapshot files for 90 days, and the timestamp for 7 days.
- One `root.json` is built into the binary. It is the starting point of trust, and it must be signed by its own root keys. No environment variable or config file can add or replace it. A registry mirror serves files, but the CLI still checks them against this root.
- The targets file lists `index.json` with its length and SHA-256 hash. The CLI downloads the index and refuses it unless it has that length and hash.
- Metadata or an index that fails a signature, threshold, hash or length check is refused, and the CLI never falls back to an unverified copy.
- The index has its own expiry date and version. An expired index is refused.
- The CLI remembers the newest index version it accepted, and it refuses an older one. It also refuses metadata that is older than what it has already seen, so a copy of an old index cannot roll your machine back to a version that was since revoked.
- Revoked versions are listed in the index. A revoked version does not run, and the CLI stops it at the next index refresh.

A build of the CLI that has no root built in cannot verify any index. It stops with the message `no trusted registry keys are embedded in this build`.

### Root rotation

The registry can replace the keys of any role without a new CLI release. To do that, it publishes a new version of the root. The new root must be signed by enough keys of the previous root and by enough keys of its own, so that nobody can install keys without the old keys agreeing. The registry keeps every version of the root. On its next refresh, the CLI walks from the root it has to the newest one, checks each step, and stores the roots it accepted. This works for the targets, snapshot and timestamp keys and for the root keys.

### Offline use and the saved copy

The CLI keeps the last verified index on disk, and installed plugins can keep running from it when the registry cannot be refreshed.

- The CLI does not trust the saved copy just because it is there. Each time it reads the saved copy, and without the network, it checks the chain of roots from the root built into the CLI, the signatures of the targets file, and the length and hash of the index. It then checks the schema, the expiry of the root, the targets file and the index, and that the index is not older than the newest one it accepted. Only the expiry of the timestamp and of the snapshot file is not checked at that point, because those files are short lived and there is no network to renew them. Signatures are always checked.
- When a refresh fails because the registry cannot be reached, because it serves older metadata or an older index, because another auth0 process holds the lock of the cache, or because the timestamp it serves has expired, the CLI keeps using the last verified index. It shows a warning with the age of the index.
- The time of the last refresh does not move in those cases. After 30 days without a good refresh, installed plugins stop until the index is refreshed.
- A signature, threshold, hash or length failure never falls back to the saved copy. An expired root, snapshot or targets file does not either, and neither does a cancelled command.

## Hashes and the lockfile

Nothing is unpacked or made executable until its SHA-256 hash matches the signed index.

- Downloads come only from `github.com`, `objects.githubusercontent.com` and `release-assets.githubusercontent.com`, over HTTPS on port 443. The CLI checks the host again on every redirect.
- The download size is capped. The default cap is 256 MiB, and the index can set a different one.
- An archive must hold exactly two files: the executable, named after the plugin (`checkmate`, or `checkmate.exe` on Windows), and `plugin.json`. Directories, links, duplicate names and names with path separators or `..` are refused.
- `plugins.lock.json` records the hash of the executable, the hash of `plugin.json`, the hash of the archive, the permissions hash, the active version and the pin. It is written with mode `0600`.
- Before every run the CLI hashes the executable and `plugin.json`. It also refuses a file that is a symlink, a file with more than one hard link, or a directory on the path that anyone else can write to. On Windows the CLI skips the mode and link checks, because Windows uses access control lists instead.

If you see a message that the installed copy "no longer matches what was verified", the file changed after the install. Reinstall the plugin with `auth0 plugin install <name>`.

## Consent for each tenant

Before a plugin first runs on a tenant, a person has to approve its permissions for that tenant. The approval is recorded in `approvals.json`, with mode `0600`.

- An approval is keyed to the tenant, the plugin and a hash of the permissions: the scopes, hosts, files, environment variables, and each command's name and whether it is interactive. The version is not part of the key. A new release with the same permissions does not ask again.
- A release with different permissions asks again. The update screen shows only what changed, and the old version stays active until the person approves.
- Only a person at a terminal can approve. Agent mode, `--no-input` and a session without a terminal all refuse to approve. `auth0 plugin approve` refuses in agent mode too.
- CI can pin the permissions hash with `AUTH0_CLI_PLUGIN_APPROVALS`, for example `auth0/checkmate@sha256:<hash>`. A pin is read strictly and held in memory for the run. The CLI never writes a pin to disk.
- Logging out of a tenant deletes that tenant's approvals before it removes the tenant. Uninstalling a plugin deletes its approvals on every tenant.

## Scope enforcement through the loopback proxy

Loopback means this machine only. For each run the CLI starts a proxy on `127.0.0.1`, an address that only programs on your machine can reach, at a random port. The proxy has a new self-signed certificate, valid for `127.0.0.1` only, and a new random secret. The plugin gets the proxy address as `AUTH0_PLUGIN_DOMAIN` and the secret as `AUTH0_PLUGIN_MANAGEMENT_TOKEN`. The plugin's SDK sends the secret as its bearer token, and the proxy swaps in the real token before it forwards the call.

The proxy denies every call unless all of these hold:

- The path is already in canonical form, which means one plain spelling of the path, and it starts with `/api/v2`. The proxy never repairs a path. A path with dot segments (`.` or `..` between slashes), encoded slashes (`%2F`), encoded dots (`%2E`) or other escapes is refused with `bad_request`. A canonical path outside `/api/v2` returns `forbidden_path`.
- The method is `GET`, `POST`, `PUT`, `PATCH` or `DELETE`. Any other method returns `method_not_allowed`.
- The endpoint is in the scope table in the signed index. An endpoint that is not in the table returns `no_scope_rule`.
- One of the scopes that allow the endpoint is among the scopes of the command you ran. If none is, the proxy returns a 403 with `insufficient_scope` and names the scope to add.

The scopes come from the command that runs, not from the whole plugin. The consent screen shows the union of all commands, but a call to one command cannot use the scopes of another.

Other protections on the proxy:

- Only `Accept`, `Content-Type`, `User-Agent` and `Auth0-Client` are forwarded. The plugin's `Authorization` header and cookies are dropped.
- A response must be JSON for the proxy to inspect it. Other responses return `unsupported_response`. A response larger than 32 MiB is refused.
- The proxy does not follow redirects. A redirect from the tenant returns `upstream_redirect`.
- The proxy refreshes an expired token at most three times in one run.
- A user login cannot be renewed silently. The proxy returns `session_expired` and tells the plugin, and you, to run `auth0 login`.

## Secret fields are removed

Before a response reaches the plugin, the proxy removes every field whose name is in the secret list from the signed index. The match ignores case, and it works at any depth in the JSON. A client secret, for example, is removed even when the plugin asks for it by name. The list lives in the index, so it can grow without a CLI release.

## What the plugin environment contains

A plugin does not inherit your environment. It gets a short list of variables, and nothing else.

- The base list: `PATH`, `HOME`, `LANG`, `LC_ALL`, `LC_CTYPE`, `TERM`, `TMPDIR`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, and the lowercase forms of the proxy variables. On Windows the list also has `SystemRoot`, `TEMP`, `TMP`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `PATHEXT` and `ComSpec`.
- The proxy and certificate variables: `AUTH0_PLUGIN_DOMAIN`, `AUTH0_PLUGIN_MANAGEMENT_TOKEN`, `AUTH0_PLUGIN_CA_CERT`, `SSL_CERT_FILE` and `NODE_EXTRA_CA_CERTS`. The last two point to the run's CA file, so the usual SDKs trust the proxy.
- The CLI flags, as `AUTH0_PLUGIN_TENANT` and one `AUTH0_PLUGIN_<FLAG>` for each CLI flag, such as `AUTH0_PLUGIN_JSON` or `AUTH0_PLUGIN_AGENT_MODE`.
- The variables that the command declares in `permissions.env`. The manifest rejects any name that starts with `AUTH0_`, and the runner drops those names too.

The plugin never receives `AUTH0_CLIENT_SECRET`, `AUTH0_API_TOKEN`, the real access token, or any other `AUTH0_` variable from your shell. The loopback addresses are added to `NO_PROXY`, so the plugin reaches the proxy directly, even when you have a corporate proxy set.

## Timeouts and the process tree

Each command has a timeout. The default is 10 minutes, and the manifest can set its own. When the timeout passes, or when you press Ctrl-C or the CLI gets SIGTERM:

- On macOS and Linux, a plugin that is not interactive runs in its own process group. The CLI sends SIGTERM to the whole group, waits 5 seconds, and then sends SIGKILL. Helper processes that the plugin started go down with it.
- On Windows, the plugin and everything it starts belong to a job object. The CLI ends the job object. Stopping on Windows is a best effort attempt.
- An interactive command stays in the CLI's process group, so the terminal keeps working. On macOS and Linux a timeout then stops only the plugin process itself.
- The proxy closes after every run, including a failed one, and its secret and certificate stop working at that point.

A plugin that times out or is interrupted ends with the reason `plugin_timeout` or `plugin_interrupted`, and the CLI exits 1, or 130 when you pressed Ctrl-C.

## What a plugin cannot do

- Read your access token or client secret from the CLI, from its environment, from its arguments or from a file the CLI writes.
- Call an endpoint that is not in the scope table, or that the scopes of the running command do not allow.
- Call a path outside `/api/v2`, such as the token endpoint.
- Read a secret field from a response, such as a client secret.
- Run on a tenant that nobody approved, or run on a session for a different tenant than the one approved.
- Run with a binary or manifest that changed after the install, or run a version that the index revoked.
- Keep calling the tenant after the run ends, because the proxy and its secret are gone.

## What a plugin can still do

A plugin runs as you, with your permissions. These limits are real, and the CLI does not stop them today:

- It can read and write any file that you can read and write. The `files` entries in the manifest are declarations that the consent screen shows, and the CLI does not enforce them.
- It can send any data it can read to any host. The `hosts` entries are declarations too, and the CLI does not enforce them.
- Because it runs as you, it could read the credentials that the CLI stores in the operating system keyring, or run `auth0` commands with your saved login. A plugin that does this would bypass the proxy. Review of the plugin before it is published is the main control against this.
- Anything it prints goes straight to your terminal. The CLI does not filter the output of a plugin.

Tokens scoped to a single run, with only the scopes that run needs, are planned. With them, a plugin would no longer use the same kind of session as the CLI. Until then, the proxy and the declared scopes are the limit.

## Telemetry

The CLI sends a usage event for plugin activity, unless you turn analytics off with `AUTH0_CLI_ANALYTICS=false`. The events are `CLI - Plugin - Run`, `Install`, `Update` and `Uninstall`.

The events carry:

- The plugin name, without the registry, and the plugin version.
- The CLI version, the operating system and the CPU architecture.
- For a run, whether it succeeded. For a failed run, the reason code of the CLI, or `other` when the code is not one of the known plugin codes.
- The install ID that the CLI sends with its other events.

The events never carry the arguments, the flags, the tenant name, the output of the plugin, file paths, or any token or secret.

The CLI sends nothing for a plugin that comes from a registry other than the built-in one, for a linked build from `plugin dev link`, or for a run that stops before the installed files are verified. A debug build with no version does not send events either.

## Reporting a problem

- For a security problem in the Auth0 CLI, do not open a public GitHub issue. Report it through the [Auth0 Responsible Disclosure Program](https://auth0.com/responsible-disclosure-policy), as the [README](https://github.com/auth0/auth0-cli#vulnerability-reporting) describes.
- For a problem with a plugin, use the bug link in that plugin's `owner.bugs` field. The author of the plugin owns the fix.
- For a bug in the CLI that is not a security problem, open an issue on the [auth0-cli repository](https://github.com/auth0/auth0-cli/issues).

## Next

- [Status](index.md#status): which parts of the registry and release path are in place.
- [Architecture](architecture.md) shows how the parts fit together and which files the CLI writes.
- [Reference](reference.md) lists the environment variables, reason codes and paths.
- [Agent mode and automation](automation.md) covers approval pins and non-interactive runs.

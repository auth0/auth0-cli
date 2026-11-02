---
layout: default
title: Plugins
has_children: true
nav_order: 8
---

Plugins add commands to the Auth0 CLI. You can install a plugin from the Auth0 registry and run it as `auth0 <name>`, or build one and release it through the same registry. The plugin commands are Experimental, so their details can change between releases.

Plugins are Experimental, and a few parts of the registry and release path are still being built. The current state is on this page under [Status](#status).

## What a plugin is

A plugin is a program that the CLI downloads, checks and starts. The CLI is the only supported way to start it. The CLI checks your approval and gives the plugin a proxy session, so running the plugin binary directly skips those checks and gives it no Auth0 access.

Each plugin has a `plugin.json` manifest. The manifest lists the plugin's commands, and for each command, the scopes of the Auth0 Management API (the API that reads and changes your tenant) that it uses. It also lists the other hosts the plugin contacts, the files it reads or writes, and the environment variables it receives. The CLI shows you this list before you approve the plugin.

## Who it is for

If you use the CLI, you can add a command that someone else built, such as a security audit, and decide what it may do on each tenant. If you build plugins, you can ship a command that uses the CLI's login, so you do not write your own token handling. If you run the CLI in CI or with an AI agent (a program that runs CLI commands for you), you can install and run plugins without a person at the keyboard, as long as a person approves the permissions first.

## The short version

```bash
auth0 plugin install checkmate
auth0 checkmate
```

The first command shows you what the plugin asks for, and it installs the plugin after you approve it for your tenant. The second command runs the plugin. Before every run, the CLI checks the installed files, the signed registry index and your approval. It then starts the plugin with a local proxy that holds your login. The plugin gets only a temporary address and token, and the proxy allows only the scopes that the command declares.

## Status

Plugins are Experimental, so commands, output and file formats can change between releases. The rest of the design is in place, and these parts are still being built:

- **Registry root.** The CLI checks the signed registry metadata against a root of trust built into the binary. That root is not in the CLI yet, so no build can install or run a plugin from the registry. A linked build stops at the same check. Plugins open to the public once the root is in place.
- **Signed releases.** The release workflows do not sign archives or record build attestations yet. The CLI will verify both once they are added.
- **Catalog.** A plugin must be listed in the registry catalog before `auth0 plugin install` can find it. A GitHub release alone does not list it.
- **Per-run tokens.** Plugins currently use the same kind of session as the CLI, through the local proxy, and the proxy limits them to the scopes each command declares. Tokens scoped to a single run are planned.

## Pages in this section

- [Getting started](getting-started.md): find a plugin, install it, approve it and run it.
- [Managing plugins](managing.md): list, inspect, pin, update, roll back, approve tenants, uninstall, and find the files on disk.
- [Building a plugin](building.md): create a project, add commands, link a local build and validate the manifest.
- [Releasing a plugin](releasing.md): set the version, check the release archive, and what the release workflow does.
- [Architecture](architecture.md): the components, the install and run flows, and the files on disk, with an interactive diagram.
- [Security and trust model](security.md): how the registry, the hashes, the approvals and the proxy protect you, and what a plugin cannot do.
- [Agent mode and automation](automation.md): what agents and CI can do, what they are refused, and how to pin an approval.
- [Troubleshooting](troubleshooting.md): the errors you can see, their reason codes and the fixes.
- [Reference](reference.md): the `plugin.json` fields, environment variables, paths, platform names and reason codes.

## Next

Start with [Getting started](getting-started.md) if you use plugins, or with [Building a plugin](building.md) if you write one.

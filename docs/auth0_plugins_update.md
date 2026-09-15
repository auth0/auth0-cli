---
layout: default
parent: auth0 plugins
has_toc: false
---
# auth0 plugins update

Update installed Auth0 CLI plugins to the version published in the signed registry. By default a plugin is updated only when the registry offers a newer version.

Run without a name to update every installed plugin, or pick one from a searchable list. Pass --all to update every installed plugin non-interactively, or --force to reinstall at the registry version even when a plugin is already up to date.

## Usage
```
auth0 plugins update [name] [flags]
```

## Examples

```
  # Pick an installed plugin to update from a searchable list
  auth0 plugins update

  # Update a specific plugin by name
  auth0 plugins update checkmate

  # Update every installed plugin
  auth0 plugins update --all

  # Reinstall a plugin at the registry version even if it is up to date
  auth0 plugins update checkmate --force
```


## Flags

```
      --all     Update every installed plugin.
      --force   Reinstall at the registry version even if the plugin is already up to date.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 plugins available](auth0_plugins_available.md) - List plugins available in the Auth0 CLI plugin registry
- [auth0 plugins install](auth0_plugins_install.md) - Install an Auth0 CLI plugin from the registry
- [auth0 plugins list](auth0_plugins_list.md) - List installed Auth0 CLI plugins
- [auth0 plugins remove](auth0_plugins_remove.md) - Remove an installed Auth0 CLI plugin
- [auth0 plugins update](auth0_plugins_update.md) - Update installed Auth0 CLI plugins



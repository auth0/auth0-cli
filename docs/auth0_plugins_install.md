---
layout: default
parent: auth0 plugins
has_toc: false
---
# auth0 plugins install

Install an Auth0 CLI plugin from the signed registry. npm plugins are run on demand through the pinned npx spec; github-release plugins are downloaded and checksum-verified for your OS and architecture.

Run without a name to pick a plugin from a searchable list of what the registry offers.

## Usage
```
auth0 plugins install [name] [flags]
```

## Examples

```
  # Pick a plugin to install from a searchable list
  auth0 plugins install

  # Install a specific plugin by name
  auth0 plugins install checkmate
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



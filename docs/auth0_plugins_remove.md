---
layout: default
parent: auth0 plugins
has_toc: false
---
# auth0 plugins remove

Remove an installed Auth0 CLI plugin from this machine.

Run without a name to pick a plugin to remove from a searchable list of what is installed.

## Usage
```
auth0 plugins remove [name] [flags]
```

## Examples

```
  # Pick an installed plugin to remove from a searchable list
  auth0 plugins remove

  # Remove a specific plugin by name
  auth0 plugins remove checkmate
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



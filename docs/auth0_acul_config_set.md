---
layout: default
parent: auth0 acul config
has_toc: false
---
# auth0 acul config set

Set the rendering settings for a specific screen.

Provide the config with `--file` (or its alias `--data`), which accepts inline JSON, an `@file.json` reference, or a file path.

Use '--schema' to print the request payload schema and exit.

## Usage
```
auth0 acul config set [flags]
```

## Examples

```
  auth0 acul config set <screen-name>
  auth0 acul config set <screen-name> --file settings.json
  auth0 acul config set signup-id --file settings.json
  auth0 acul config set login-id
  auth0 acul config set signup-id --data '{"rendering_mode":"advanced"}'
  auth0 acul config set signup-id --data @settings.json
  auth0 acul config set --schema
```


## Flags

```
      --data string   Alias for --file. Rendering config as inline JSON, an @file.json reference, or a file path.
  -f, --file string   File to save the rendering configs to.
      --schema        Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 acul config docs](auth0_acul_config_docs.md) - Open the ACUL configuration documentation
- [auth0 acul config generate](auth0_acul_config_generate.md) - Generate a stub config file for a Universal Login screen.
- [auth0 acul config get](auth0_acul_config_get.md) - Get the current rendering settings for a specific screen
- [auth0 acul config list](auth0_acul_config_list.md) - List Universal Login rendering configurations
- [auth0 acul config set](auth0_acul_config_set.md) - Set the rendering settings for a specific screen



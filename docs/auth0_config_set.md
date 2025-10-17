---
layout: default
parent: auth0 config
has_toc: false
---
# auth0 config set

Set the rendering settings for a specific screen.

## Usage
```
auth0 config set [flags]
```

## Examples

```
  auth0 acul config set signup-id --file settings.json
  auth0 acul config set login-id
```


## Flags

```
  -f, --file string   File to save the rendering configs to.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 config docs](auth0_config_docs.md) - Open the ACUL configuration documentation
- [auth0 config generate](auth0_config_generate.md) - Generate a stub config file for a Universal Login screen.
- [auth0 config get](auth0_config_get.md) - Get the current rendering settings for a specific screen
- [auth0 config list](auth0_config_list.md) - List Universal Login rendering configurations
- [auth0 config set](auth0_config_set.md) - Set the rendering settings for a specific screen



---
layout: default
parent: auth0 acul config
has_toc: false
---
# auth0 acul config get

Get the current rendering settings for a specific screen.

## Usage
```
auth0 acul config get [flags]
```

## Examples

```
  auth0 acul config get <screen-name>
  auth0 acul config get <screen-name> --file settings.json
  auth0 acul config get signup-id
  auth0 acul config get login-id -f ./acul_config/login-id.json
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

- [auth0 acul config docs](auth0_acul_config_docs.md) - Open the ACUL configuration documentation
- [auth0 acul config generate](auth0_acul_config_generate.md) - Generate a stub config file for a Universal Login screen.
- [auth0 acul config get](auth0_acul_config_get.md) - Get the current rendering settings for a specific screen
- [auth0 acul config list](auth0_acul_config_list.md) - List Universal Login rendering configurations
- [auth0 acul config set](auth0_acul_config_set.md) - Set the rendering settings for a specific screen



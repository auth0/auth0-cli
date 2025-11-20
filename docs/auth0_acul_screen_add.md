---
layout: default
parent: auth0 acul screen
has_toc: false
---
# auth0 acul screen add

Add screens to an existing project. The project must have been initialized using `auth0 acul init`.

## Usage
```
auth0 acul screen add [flags]
```

## Examples

```
  auth0 acul screen add <screen-name> <screen-name>... --dir <app-directory>
  auth0 acul screen add login-id login-password -d acul_app
```


## Flags

```
  -d, --dir acul_config.json   Path to existing project directory (must contain acul_config.json)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 acul screen add](auth0_acul_screen_add.md) - Add screens to an existing project



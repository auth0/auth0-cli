---
layout: default
parent: auth0 universal-login templates
has_toc: false
---
# auth0 universal-login templates update

Update the custom template for the New Universal Login Experience.

## Usage
```
auth0 universal-login templates update [flags]
```

## Examples

```
  auth0 universal-login templates update
  auth0 ul templates update
  cat login.liquid | auth0 ul templates update
  echo "<html>{%- auth0:head -%}{%- auth0:widget -%}</html>" | auth0 ul templates update
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-login templates show](auth0_universal-login_templates_show.md) - Display the custom template for Universal Login
- [auth0 universal-login templates update](auth0_universal-login_templates_update.md) - Update the custom template for Universal Login



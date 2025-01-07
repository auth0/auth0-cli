---
layout: default
parent: auth0 email provider
has_toc: false
---
# auth0 email provider delete

Delete the email provider.

To delete interactively, use `auth0 email provider delete` with no arguments.

To delete non-interactively, supply the the `--force` flag to skip confirmation.

## Usage
```
auth0 email provider delete [flags]
```

## Examples

```
  auth0 provider delete
  auth0 email provider rm
  auth0 email provider delete --force
  auth0 email provider rm --force
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

- [auth0 email provider create](auth0_email_provider_create.md) - Create the email provider
- [auth0 email provider delete](auth0_email_provider_delete.md) - Delete the email provider
- [auth0 email provider show](auth0_email_provider_show.md) - Show the email provider
- [auth0 email provider update](auth0_email_provider_update.md) - Update the email provider



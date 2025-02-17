---
layout: default
parent: auth0 phone provider
has_toc: false
---
# auth0 phone provider delete

Delete the phone provider.

To delete interactively, use `auth0 phone provider delete` with no arguments.

To delete non-interactively, supply the phone provider id and the `--force` flag to skip confirmation.

## Usage
```
auth0 phone provider delete [flags]
```

## Examples

```
auth0 provider delete
auth0 phone provider rm
auth0 phone provider delete <phone-provider-id> --force
auth0 phone provider delete <phone-provider-id>
auth0 phone provider rm --force
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

- [auth0 phone provider create](auth0_phone_provider_create.md) - Create the phone provider
- [auth0 phone provider delete](auth0_phone_provider_delete.md) - Delete the phone provider
- [auth0 phone provider show](auth0_phone_provider_show.md) - Show the Phone provider
- [auth0 phone provider update](auth0_phone_provider_update.md) - Update the phone provider



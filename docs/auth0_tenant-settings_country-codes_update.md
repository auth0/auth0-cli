---
layout: default
parent: auth0 tenant-settings country-codes
has_toc: false
---
# auth0 tenant-settings country-codes update

Set country codes filtering for the tenant.

To set country codes interactively, omit the flags.

## Usage
```
auth0 tenant-settings country-codes update [flags]
```

## Examples

```
  auth0 tenant-settings country-codes update --list US,GB,CA --mode allow
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --list string    Comma-separated ISO 3166-1 alpha-2 country codes (e.g., US,GB,CA).
      --mode string    Filter mode for country codes. One of allow or deny.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 tenant-settings country-codes remove](auth0_tenant-settings_country-codes_remove.md) - Remove country codes filtering from the tenant
- [auth0 tenant-settings country-codes show](auth0_tenant-settings_country-codes_show.md) - Display the tenant's country codes filtering
- [auth0 tenant-settings country-codes update](auth0_tenant-settings_country-codes_update.md) - Set country codes filtering for the tenant



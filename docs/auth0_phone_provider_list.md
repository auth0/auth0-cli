---
layout: default
parent: auth0 phone provider
has_toc: false
---
# auth0 phone provider list

List your existing Phone providers. Currently we can create a max of 1 phone Provider, If none are created, you can create one by running `auth0 phone provider create`.

## Usage
```
auth0 phone provider list [flags]
```

## Examples

```
  auth0 phone provider list
  auth0 phone provider ls 
  auth0 phone provider ls --json
  auth0 phone provider ls --csv
```


## Flags

```
      --csv    Output in csv format.
      --json   Output in json format.
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
- [auth0 phone provider list](auth0_phone_provider_list.md) - List your Phone providers
- [auth0 phone provider show](auth0_phone_provider_show.md) - Show the Phone provider
- [auth0 phone provider update](auth0_phone_provider_update.md) - Update the phone provider



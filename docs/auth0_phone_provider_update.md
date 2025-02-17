---
layout: default
parent: auth0 phone provider
has_toc: false
---
# auth0 phone provider update

Update the phone provider.

To update interactively, use `auth0 phone provider update` with no arguments.

To update non-interactively, supply the provider name and other information through the flags.

## Usage
```
auth0 phone provider update [flags]
```

## Examples

```
  auth0 phone provider update
  auth0 phone provider update --json
  auth0 phone provider update --disabled=false
  auth0 phone provider update --credentials='{ "auth_token":"NewAuthToken" }'
  auth0 phone provider update --configuration='{ "delivery_methods": ["voice"] }'
  auth0 phone provider update --configuration='{ "default_from": admin@example.com }'
  auth0 phone provider update --provider twilio --disabled=false --credentials='{ "auth_token":"NewAuthToken" }' --configuration='{ "default_from": "admin@example.com", "delivery_methods": ["voice", "text"] }'
  auth0 phone provider update --provider custom --disabled=false --configuration='{ "delivery_methods": ["voice", "text"] }"
```


## Flags

```
  -s, --configuration string   Configuration for the phone provider. formatted as JSON.
  -c, --credentials string     Credentials for the phone provider, formatted as JSON.
  -d, --disabled               Whether the provided is disabled (true) or enabled (false).
      --json                   Output in json format.
  -p, --provider string        Provider name. Can be 'twilio', or 'custom'
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



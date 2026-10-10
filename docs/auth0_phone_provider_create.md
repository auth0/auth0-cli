---
layout: default
parent: auth0 phone provider
has_toc: false
---
# auth0 phone provider create

Create the phone provider.

To create interactively, use `auth0 phone provider create` with no arguments.

To create non-interactively, supply the provider name and other information through the flags.

Use `--schema` to print the request payload schema and exit.

## Usage
```
auth0 phone provider create [flags]
```

## Examples

```
  auth0 phone provider create
  auth0 phone provider create --json
  auth0 phone provider create --json-compact
  auth0 phone provider create --provider twilio --disabled=false --credentials='{ "auth_token":"TheAuthToken" }' --configuration='{ "default_from": "admin@example.com", "sid": "+1234567890", "delivery_methods": ["text", "voice"] }'
  auth0 phone provider create --provider custom --disabled=true --configuration='{ "delivery_methods": ["text", "voice"] }'
  auth0 phone provider create -p twilio -d "false" -c '{ "auth_token":"TheAuthToken" }' -s '{ "default_from": "admin@example.com", "sid": "+1234567890", "delivery_methods": ["text"] }'

  # Discover the payload schema
  auth0 phone provider create --schema
  auth0 phone provider create --schema --json

  # JSON input mode (for agents and automation)
  auth0 phone provider create --data '{"name":"twilio","credentials":{"auth_token":"TheAuthToken"},"configuration":{"default_from":"+1234567890","delivery_methods":["text"]}}'
  auth0 phone provider create --data @provider.json
```


## Flags

```
  -s, --configuration string   Configuration for the phone provider. formatted as JSON.
  -c, --credentials string     Credentials for the phone provider, formatted as JSON.
      --data string            JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --disabled               Whether the provided is disabled (true) or enabled (false).
      --json                   Output in json format.
      --json-compact           Output in compact json format.
  -p, --provider string        Provider name. Can be 'twilio', or 'custom'
      --schema                 Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 phone provider create](auth0_phone_provider_create.md) - Create the phone provider
- [auth0 phone provider delete](auth0_phone_provider_delete.md) - Delete the phone provider
- [auth0 phone provider list](auth0_phone_provider_list.md) - List your Phone providers
- [auth0 phone provider show](auth0_phone_provider_show.md) - Show the Phone provider
- [auth0 phone provider update](auth0_phone_provider_update.md) - Update the phone provider



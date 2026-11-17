---
layout: default
parent: auth0 token-exchange
has_toc: false
---
# auth0 token-exchange create

Create a new token exchange profile.

To create interactively, use `auth0 token-exchange create` with no flags.

To create non-interactively, supply the name, subject token type, action ID, and type through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 token-exchange create [flags]
```

## Examples

```
  auth0 token-exchange create
  auth0 token-exchange create --name "My Token Exchange Profile"
  auth0 token-exchange create --name "My Token Exchange Profile" --subject-token-type "urn:ietf:params:oauth:token-type:jwt"
  auth0 token-exchange create --name "My Token Exchange Profile" --subject-token-type "urn:ietf:params:oauth:token-type:jwt" --action-id "act_123abc" --type "custom_authentication"
  auth0 token-exchange create -n "My Token Exchange Profile" -s "urn:ietf:params:oauth:token-type:jwt" -a "act_123abc" -t "custom_authentication" --json
  auth0 token-exchange create -n "My Token Exchange Profile" -s "urn:ietf:params:oauth:token-type:jwt" -a "act_123abc" -t "custom_authentication" --json-compact

  # Discover the payload schema
  auth0 token-exchange create --schema
  auth0 token-exchange create --schema --json

  # JSON input mode (for agents and automation)
  auth0 token-exchange create --data '{"name":"My Profile","subject_token_type":"urn:acme:legacy-token","action_id":"act_123abc","type":"custom_authentication"}'
  auth0 token-exchange create --data @profile.json
  cat profile.json | auth0 token-exchange create
```


## Flags

```
  -a, --action-id string            Identifier of the action.
      --data string                 JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -n, --name string                 Name of the token exchange profile.
      --schema                      Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --subject-token-type string   Type of the subject token. Must be a valid URI format (e.g., urn:ietf:params:oauth:token-type:jwt). Cannot use reserved prefixes: http://auth0.com, https://auth0.com, http://okta.com, https://okta.com, urn:ietf, urn:auth0, urn:okta.
  -t, --type string                 Type of the token exchange profile. Currently only 'custom_authentication' is supported.
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

- [auth0 token-exchange create](auth0_token-exchange_create.md) - Create a new token exchange profile
- [auth0 token-exchange delete](auth0_token-exchange_delete.md) - Delete a token exchange profile
- [auth0 token-exchange list](auth0_token-exchange_list.md) - List your token exchange profiles
- [auth0 token-exchange show](auth0_token-exchange_show.md) - Show a token exchange profile
- [auth0 token-exchange update](auth0_token-exchange_update.md) - Update a token exchange profile



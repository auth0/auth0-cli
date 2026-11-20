---
layout: default
parent: auth0 network-acl
has_toc: false
---
# auth0 network-acl update

Update a network ACL.
To update interactively, use "auth0 network-acl update" with no arguments.
To update non-interactively, supply the description, active, priority, and rule through flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 network-acl update [flags]
```

## Examples

```
  auth0 network-acl update <id>
  auth0 network-acl update <id> --priority 5
  auth0 network-acl update <id> --active true
  auth0 network-acl update <id> --description "Updated description"
  auth0 network-acl update <id> --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl update <id> --description "Complex Rule updated" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'
  auth0 network-acl update <id> --rule '{"action":{"block":true},"scope":"tenant","match_all":true}'

  # Early Access (auth0_managed and http_message_signature match/not_match value):
  auth0 network-acl update <id> --rule '{"action":{"allow":true},"scope":"tenant","match":{"auth0_managed":["auth0.low_reputation"]}}'
  auth0 network-acl update <id> --rule '{"action":{"allow":true},"scope":"authentication","match":{"http_message_signature":{"keys":[{"id": "key_123"},{"id": "key_456"}]}}}'

  # Discover the payload schema
  auth0 network-acl update --schema
  auth0 network-acl update --schema --json

  # JSON input mode (for agents and automation)
  auth0 network-acl update <id> --data '{"active":false,"priority":5}'
  auth0 network-acl update <id> --data @network-acl.json
  cat network-acl.json | auth0 network-acl update <id>
  
```


## Flags

```
      --active string        Whether the network ACL is active ('true' or 'false').
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --description string   Description of the network ACL (Eg. "Block suspicious IPs").
      --json                 Output in JSON format
      --json-compact         Output in compact json format.
  -p, --priority int         Priority of the network ACL (Eg. 5). (default 1)
      --rule string          Network ACL rule configuration in JSON format (required for non-interactive mode).
      --schema               Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 network-acl create](auth0_network-acl_create.md) - Create a new network ACL
- [auth0 network-acl delete](auth0_network-acl_delete.md) - Delete a network ACL
- [auth0 network-acl list](auth0_network-acl_list.md) - List network ACLs
- [auth0 network-acl show](auth0_network-acl_show.md) - Show a network ACL
- [auth0 network-acl update](auth0_network-acl_update.md) - Update a network ACL



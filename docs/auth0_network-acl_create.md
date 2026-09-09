---
layout: default
parent: auth0 network-acl
has_toc: false
---
# auth0 network-acl create

Create a new network ACL.
To create interactively, use "auth0 network-acl create" with no arguments.
To create non-interactively, supply the required parameters (description, active, priority, and rule) through flags.
The --rule parameter is required and must contain a valid JSON object with action, scope, and match properties.

## Usage
```
auth0 network-acl create [flags]
```

## Examples

```
  auth0 network-acl create
  auth0 network-acl create --description "Block IPs" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24","10.0.0.0/8"]}}'
  auth0 network-acl create --description "Geo Block" --priority 2 --active true --rule '{"action":{"block":true},"scope":"authentication","match":{"geo_country_codes":["US","CA"]}}'
  auth0 network-acl create --description "Redirect Traffic" --priority 3 --active true --rule '{"action":{"redirect":true,"redirect_uri":"https://example.com"},"scope":"management","match":{"ipv4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl create -d "Block Bots" -p 4 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"user_agents":["badbot/*","malicious/*"],"ja3_fingerprints":["deadbeef","cafebabe"]}}'
  auth0 network-acl create --description "Complex Rule" --priority 5 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'
  auth0 network-acl create --description "Deny All" --priority 7 --active true --rule '{"action":{"block":true},"scope":"tenant","match_all":true}'

  # Early Access (auth0_managed and http_message_signature match/not_match value):
  auth0 network-acl create -d "Curated Blocklist" -p 6 --active true --rule '{"action":{"log":true},"scope":"tenant","not_match":{"auth0_managed":["auth0.vpn","auth0.proxy"]}}'
  auth0 network-acl create -d "Only Signed" -p 8 --active true --rule '{"action":{"allow":true},"scope":"authentication","match":{"http_message_signature":{"keys":[{"id": "key_123"}]}}}'
  
```


## Flags

```
      --active string        Whether the network ACL is active ('true' or 'false').
  -d, --description string   Description of the network ACL (Eg. "Block suspicious IPs").
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -p, --priority int         Priority of the network ACL (Eg. 5).
      --rule string          Network ACL rule configuration in JSON format (required for non-interactive mode).
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



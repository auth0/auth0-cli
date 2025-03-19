---
layout: default
parent: auth0 network-acl
has_toc: false
---
# auth0 network-acl update

Update a network ACL.
To update interactively, use "auth0 network-acl update" with no arguments.
To update non-interactively, supply the required parameters (description, active, priority, and rule) through flags.
When updating the rule, provide a complete JSON object with action, scope, and match/not_match properties.

## Usage
```
auth0 network-acl update [flags]
```

## Examples

```
  auth0 network-acl update <id>
  auth0 network-acl update <id> --priority 5 
  auth0 network-acl update <id> --active true
  auth0 network-acl update <id> --description "Complex Rule updated" --priority 9 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ip_v4_cidrs":["192.168.1.0/24"],"country_codes":["US"]}}'
```


## Flags

```
      --action string               Action for the rule (block, allow, log, redirect)
      --active string               Whether the network ACL is active ('true' or 'false')
      --anonymous-proxy             Match anonymous proxy traffic
      --asns ints                   Comma-separated list of ASNs to match
      --country-codes strings       Comma-separated list of country codes to match
  -d, --description string          Description of the network ACL
      --ipv4-cidrs strings          Comma-separated list of IPv4 CIDR ranges
      --ipv6-cidrs strings          Comma-separated list of IPv6 CIDR ranges
      --ja3-fingerprints strings    Comma-separated list of JA3 fingerprints to match
      --ja4-fingerprints strings    Comma-separated list of JA4 fingerprints to match
  -p, --priority int                Priority of the network ACL (1-10) (default 1)
      --redirect-uri string         URI to redirect to when action is redirect
      --rule string                 Network ACL rule configuration in JSON format
      --scope string                Scope of the rule (management, authentication, tenant)
      --subdivision-codes strings   Comma-separated list of subdivision codes to match
      --user-agents strings         Comma-separated list of user agents to match
```


## Inherited Flags

```
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



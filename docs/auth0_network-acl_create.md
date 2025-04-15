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
  auth0 network-acl create --description "Geo Block" --priority 2 --active true --rule '{"action":{"block":true},"scope":"authentication","match":{"country_codes":["US","CA"],"anonymous_proxy":true}}'
  auth0 network-acl create --description "Redirect Traffic" --priority 3 --active true --rule '{"action":{"redirect":true,"redirect_uri":"https://example.com"},"scope":"management","match":{"ipv4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl create -d "Block Bots" -p 4 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"user_agents":["badbot/*","malicious/*"],"ja3_fingerprints":["deadbeef","cafebabe"]}}'
  auth0 network-acl create --description "Complex Rule" --priority 5 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"country_codes":["US"]}}'
```


## Flags

```
      --action string               Action for the rule (block, allow, log, redirect)
      --active string               Whether the network ACL is active (required, 'true' or 'false')
      --asns ints                   Comma-separated list of ASNs to match (Eg. 64496,64497,64498)
      --country-codes strings       Comma-separated list of country codes to match (Eg. US,CA,MX)
  -d, --description string          Description of the network ACL (required)
      --ipv4-cidrs strings          Comma-separated list of IPv4 CIDR ranges (Eg. 192.168.1.0/24,10.0.0.0/8)
      --ipv6-cidrs strings          Comma-separated list of IPv6 CIDR ranges (Eg. 2001:db8::/32,2001:db8:1234::/48)
      --ja3-fingerprints strings    Comma-separated list of JA3 fingerprints to match (Eg. deadbeef,cafebabe)
      --ja4-fingerprints strings    Comma-separated list of JA4 fingerprints to match (Eg. t13d1516h2_8daaf6152771)
  -p, --priority int                Priority of the network ACL (required, 1-10)
      --redirect-uri string         URI to redirect to when action is redirect
      --rule string                 Network ACL rule configuration in JSON format (required for non-interactive mode)
      --scope string                Scope of the rule (management, authentication, tenant)
      --subdivision-codes strings   Comma-separated list of subdivision codes to match (Eg. US-NY,US-CA)
      --user-agents strings         Comma-separated list of user agents to match (Eg. badbot/*,malicious/*)
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



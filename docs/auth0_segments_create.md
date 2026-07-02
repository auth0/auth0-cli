---
layout: default
parent: auth0 segments
has_toc: false
---
# auth0 segments create

Create a new segment.

To create interactively, use `auth0 segments create` with no flags.

To create non-interactively, supply name and rules through the flags.

## Usage
```
auth0 segments create [flags]
```

## Examples

```
  auth0 segments create
  auth0 segments create --name "Beta Users" --rules '[{"match":{"domain":{"contains":["beta.example.com"]}}}]'
  auth0 segments create -n "Internal" -r '[{"match":{"domain":{"ends_with":["mycompany.com"]}}}]'
  auth0 segments create -n "US Chrome" -r '[{"match":{"country":["US"],"browser":{"contains":["Chrome"]}}}]'
  auth0 segments create -n "External non-US" -r '[{"match":{"domain":{"ends_with":["example.com"]}},"not_match":{"country":["US"]}}]'
```


## Flags

```
  -d, --description string   Description of the segment.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the segment.
  -r, --rules match          Rules for matching users, as a JSON array. Each rule has a match and/or `not_match` object that maps an attribute to a condition.
                             Attributes: client_id, connection, connection_type, organization_id, domain, device_type, browser, platform, user_agent, country, region.
                             Conditions: contains, starts_with, ends_with, exists, or a plain list ["a","b"] for an exact match.
                             Example: '[{"match":{"domain":{"ends_with":["example.com"]}}}]'
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 segments create](auth0_segments_create.md) - Create a new segment
- [auth0 segments delete](auth0_segments_delete.md) - Delete a segment
- [auth0 segments list](auth0_segments_list.md) - List your segments
- [auth0 segments show](auth0_segments_show.md) - Show a segment
- [auth0 segments update](auth0_segments_update.md) - Update a segment



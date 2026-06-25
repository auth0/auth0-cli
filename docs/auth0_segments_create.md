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
  auth0 segments create --name "Beta Users" --rules '[{"match":{"contains":["@beta.example.com"]}}]'
  auth0 segments create -n "Internal" -r '[{"match":{"ends_with":["@mycompany.com"]}}]'
```


## Flags

```
  -d, --description string   Description of the segment.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the segment.
  -r, --rules string         Rules for matching users. JSON array. Example: '[{"match":{"contains":["@example.com"]}}]'
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



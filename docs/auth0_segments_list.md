---
layout: default
parent: auth0 segments
has_toc: false
---
# auth0 segments list

List all segments. To create one, run: `auth0 segments create`.

## Usage
```
auth0 segments list [flags]
```

## Examples

```
  auth0 segments list
  auth0 segments ls
  auth0 segments list --json
  auth0 segments list --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
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



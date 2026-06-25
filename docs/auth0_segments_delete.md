---
layout: default
parent: auth0 segments
has_toc: false
---
# auth0 segments delete

Delete a segment.

To delete interactively, use `auth0 segments delete` with no arguments.

To delete non-interactively, supply the segment ID and use `--force` to skip confirmation.

## Usage
```
auth0 segments delete [flags]
```

## Examples

```
  auth0 segments delete
  auth0 segments rm
  auth0 segments delete <segment-id>
  auth0 segments delete <segment-id> --force
  auth0 segments delete <segment-id> <segment-id2> --force
```


## Flags

```
      --force   Skip confirmation.
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



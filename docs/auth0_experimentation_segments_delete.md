---
layout: default
parent: auth0 experimentation segments
has_toc: false
---
# auth0 experimentation segments delete

Delete a segment.

To delete interactively, use `auth0 experimentation segments delete` with no arguments.

To delete non-interactively, supply the segment ID and use `--force` to skip confirmation.

## Usage
```
auth0 experimentation segments delete [flags]
```

## Examples

```
  auth0 experimentation segments delete
  auth0 experimentation segments rm
  auth0 experimentation segments delete <segment-id>
  auth0 experimentation segments delete <segment-id> --force
  auth0 experimentation segments delete <segment-id> <segment-id2> --force
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

- [auth0 experimentation segments create](auth0_experimentation_segments_create.md) - Create a new segment
- [auth0 experimentation segments delete](auth0_experimentation_segments_delete.md) - Delete a segment
- [auth0 experimentation segments list](auth0_experimentation_segments_list.md) - List your segments
- [auth0 experimentation segments show](auth0_experimentation_segments_show.md) - Show a segment
- [auth0 experimentation segments update](auth0_experimentation_segments_update.md) - Update a segment



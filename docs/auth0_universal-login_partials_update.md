---
layout: default
parent: auth0 universal-login partials
has_toc: false
---
# auth0 universal-login partials update

Update partials for a prompt segment.

## Usage
```
auth0 universal-login partials update [flags]
```

## Examples

```
	auth0 universal-login partials update <prompt>
	auth0 ul partials update <prompt> --input-file <input-file>
	auth0 ul partials update login --input-file /tmp/login/input-file.json
```


## Flags

```
      --form-content-end string          Content for the Form Content End Partial
      --form-content-start string        Content for the Form Content Start Partial
      --form-footer-end string           Content for the Form Footer End Partial
      --form-footer-start string         Content for the Form Footer Start Partial
      --input-file string                Path to a file that contains partial definitions for a prompt segment.
      --secondary-actions-end string     Content for the Secondary Actions End Partial
      --secondary-actions-start string   Content for the Secondary Actions Start Partial
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-login partials create](auth0_universal-login_partials_create.md) - Create partials for a prompt segment
- [auth0 universal-login partials delete](auth0_universal-login_partials_delete.md) - Delete partials for a prompt segment
- [auth0 universal-login partials show](auth0_universal-login_partials_show.md) - Show partials for a prompt segment
- [auth0 universal-login partials update](auth0_universal-login_partials_update.md) - Update partials for a prompt segment



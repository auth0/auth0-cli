---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms delete

Delete a form.

To delete interactively, use `auth0 forms delete` with no arguments.

To delete non-interactively, supply the form id and the `--force` flag to skip confirmation.

## Usage
```
auth0 forms delete [flags]
```

## Examples

```
  auth0 forms delete
  auth0 forms rm
  auth0 forms delete <form-id>
  auth0 forms delete <form-id> --force
  auth0 forms delete <form-id> <form-id2> <form-idn>
```


## Flags

```
      --force   Skip confirmation.
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

- [auth0 forms create](auth0_forms_create.md) - Create a new form
- [auth0 forms delete](auth0_forms_delete.md) - Delete a form
- [auth0 forms export](auth0_forms_export.md) - Export a form
- [auth0 forms import](auth0_forms_import.md) - Import a form
- [auth0 forms list](auth0_forms_list.md) - List your forms
- [auth0 forms open](auth0_forms_open.md) - Open a form in the Auth0 Dashboard
- [auth0 forms show](auth0_forms_show.md) - Show a form
- [auth0 forms update](auth0_forms_update.md) - Update a form



---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms update

Update a form.

Passing `--file` (or piped stdin) replaces every top-level field present in the file. Passing only scalar flags such as `--name` performs a merge that preserves the form's graph fields (nodes, style, translations). Server-managed fields such as `id`, `created_at`, and `updated_at` are removed before the update request is sent.

## Usage
```
auth0 forms update [flags]
```

## Examples

```
  auth0 forms update <form-id> --name "New Name"
  auth0 forms update <form-id> --file ./form.json
  cat form.json | auth0 forms update <form-id> -f -
```


## Flags

```
  -f, --file string               Path to a JSON file with the form body. Use '-' to read from stdin.
      --json                      Output in json format.
      --json-compact              Output in compact json format.
      --language-default string   Default language of the Form (e.g. en).
      --language-primary string   Primary language of the Form (e.g. en).
      --name string               Name of the Form.
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



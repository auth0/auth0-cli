---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms export

Export a form as JSON. Writes to stdout by default (pipe-friendly) or to a file with `--output`. The output uses the same envelope as the Auth0 Dashboard (`version`, `form`, `flows`, `connections`), bundling the flows and vault connections the form references with portable `#FLOW-N#`/`#CONN-N#` placeholders, so it can be imported by the CLI or opened in the Dashboard.

## Usage
```
auth0 forms export [flags]
```

## Examples

```
  auth0 forms export <form-id>
  auth0 forms export <form-id> --output ./form.json
  auth0 forms export <form-id> --json-compact
  auth0 forms export <form-id> | auth0 forms import -f -
```


## Flags

```
      --json-compact    Output in compact json format.
  -o, --output string   Path to write the exported form. Writes to stdout when omitted.
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



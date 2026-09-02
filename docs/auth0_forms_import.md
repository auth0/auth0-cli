---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms import

Import a form from a JSON file (or piped stdin). Without `--id` a new form is created; with `--id` the existing form is replaced.

Both a flat form graph and the Dashboard envelope (`version`, `form`, `flows`, `connections`) are accepted. For an envelope, the bundled flows are created and each `#CONN-N#` connection placeholder is mapped to an existing vault connection, either interactively or with `--connection`.

## Usage
```
auth0 forms import [flags]
```

## Examples

```
  auth0 forms import --file ./form.json
  auth0 forms import --file ./form.json --id <form-id>
  auth0 forms import --file ./form.json --connection '#CONN-1#=ac_123'
  cat form.json | auth0 forms import -f -
```


## Flags

```
      --connection stringToString   Map an exported connection placeholder to an existing vault connection ID, e.g. --connection '#CONN-1#=ac_123'. Repeatable. (default [])
  -f, --file string                 Path to a JSON file with the form body. Use '-' to read from stdin.
      --id string                   Id of an existing Form to replace. When omitted, a new form is created.
      --json                        Output in json format.
      --json-compact                Output in compact json format.
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



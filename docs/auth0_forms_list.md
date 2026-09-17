---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms list

List your existing forms. To create one, run: `auth0 forms create`.

## Usage
```
auth0 forms list [flags]
```

## Examples

```
  auth0 forms list
  auth0 forms ls
  auth0 forms ls --number 100
  auth0 forms ls --json
  auth0 forms ls --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of forms to retrieve. Fetched across pages. (default 100)
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



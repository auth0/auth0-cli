---
layout: default
parent: auth0 forms
has_toc: false
---
# auth0 forms create

Create a new form.

Interactive behavior: `auth0 forms create` asks only for the name and creates a minimal scaffold; it does not open an editor. You can then refine the form in the dashboard builder.

Pass `--edit` to open an editor and author the form graph before it is created, or supply the whole body via `--data` as inline JSON, a file (`@form.json`), or piped stdin. Run `auth0 forms create --schema` to print the accepted payload schema and `auth0 forms create --example > form.json` to generate a starter body.

`--data` provides the whole payload and cannot be combined with `--name` or the `--language-*` flags; it is checked for valid JSON and a form name before it is sent, and the form graph itself is validated by the API.

## Usage
```
auth0 forms create [flags]
```

## Examples

```
  auth0 forms create
  auth0 forms create --name "My Form"
  auth0 forms create --name "My Form" --edit
  auth0 forms create --example > form.json
  auth0 forms create --schema
  auth0 forms create --data '{"name":"My Form"}'
  auth0 forms create --data @form.json
  cat form.json | auth0 forms create
```


## Flags

```
      --data string               JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --edit                      Open an editor to author the form graph after entering the name.
      --example                   Print an example form JSON body and exit.
      --json                      Output in json format.
      --json-compact              Output in compact json format.
      --language-default string   Default language of the Form (e.g. en).
      --language-primary string   Primary language of the Form (e.g. en).
      --name string               Name of the Form.
      --schema                    Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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



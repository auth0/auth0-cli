---
layout: default
has_toc: false
---
# auth0 commands

List every command in a compact tree, along with a short description of what it does.

This gives you (or an AI agent) a single overview of the whole CLI surface, so the right command can be found without inspecting each `--help` page individually.

Pass a command path to expand only that branch instead of the whole tree, for example `auth0 commands apps` or `auth0 commands apps create`. This keeps the output focused when you only care about one area.

Use `--flat` to list every runnable command on its own line, which is the easiest form to scan or match an intent against. Use `--json` for a machine-readable representation, and add `--detailed` to include usage lines, flags, arguments and whether authentication is required, which is enough for an agent to construct a valid invocation on its own.

## Usage
```
auth0 commands [command]
```

## Examples

```
  auth0 commands
  auth0 commands --flat
  auth0 commands apps
  auth0 commands apps create --detailed
  auth0 commands apps --json --detailed
```


## Flags

```
      --depth int   Maximum depth to display. 0 shows all levels. Ignored with --flat.
      --detailed    Include usage, flags, arguments and auth requirements. Best used with --json.
      --flat        List every runnable command on its own line, best for scanning or intent matching.
      --json        Output in json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```



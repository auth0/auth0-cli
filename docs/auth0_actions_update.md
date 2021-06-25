---
layout: default
---
## auth0 actions update

Update an action

### Synopsis

Update an action.

```
auth0 actions update [flags]
```

### Examples

```
auth0 actions update <id> 
auth0 actions update <id> --name myaction
auth0 actions update <id> --n myaction --trigger post-login
auth0 actions update <id> --n myaction -t post-login -d "lodash=4.0.0" -d "uuid=8.0.0"
auth0 actions update <id> --n myaction -t post-login -d "lodash=4.0.0" -s "API_KEY=value" -s "SECRET=value
```

### Options

```
  -c, --code string                 Code content for the action.
  -d, --dependency stringToString   Third party npm module, and it version, that the action depends on. (default [])
  -h, --help                        help for update
  -n, --name string                 Name of the action.
  -s, --secret stringToString       Secret to be used in the action. (default [])
  -t, --trigger string              Trigger of the action. At this time, an action can only target a single trigger at a time.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 actions](auth0_actions.md)	 - Manage resources for actions


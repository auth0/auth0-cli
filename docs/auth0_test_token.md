---
layout: default
---
## auth0 test token

Fetch a token for the given application and API

### Synopsis

Fetch an access token for the given application.
If --client-id is not provided, the default client "CLI Login Testing" will be used (and created if not exists).
Specify the API you want this token for with --audience (API Identifer). Additionally, you can also specify the --scope to use.

```
auth0 test token [flags]
```

### Examples

```
auth0 test token
auth0 test token --client-id <id> --audience <audience> --scopes <scope1,scope2>
```

### Options

```
  -a, --audience string    The unique identifier of the target API you want to access.
  -c, --client-id string   Client Id of an Auth0 application.
  -h, --help               help for token
  -s, --scopes strings     The list of scopes you want to use.
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

* [auth0 test](auth0_test.md)	 - Try your Universal Login box or get a token


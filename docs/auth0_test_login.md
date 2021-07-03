---
layout: default
---
## auth0 test login

Try out your Universal Login box

### Synopsis

Launch a browser to try out your Universal Login box.

```
auth0 test login [flags]
```

### Examples

```
auth0 test login
auth0 test login <client-id>
auth0 test login <client-id> --connection <connection>
```

### Options

```
  -a, --audience string     The unique identifier of the target API you want to access.
      --connection string   Connection to test during login.
  -d, --domain string       One of your custom domains.
  -h, --help                help for login
  -s, --scopes strings      The list of scopes you want to use. (default [openid,profile])
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


---
layout: default
---
## auth0 login

Authenticate the Auth0 CLI

### Synopsis

Authenticates the Auth0 CLI either as a user using personal credentials or as a machine using client credentials (client ID/secret).

```
auth0 login [flags]
```

### Examples

```

		auth0 login
		auth0 login --as-machine
		auth0 login --as-machine --domain <TENANT_DOMAIN> --client-id <CLIENT_ID> --client-secret <CLIENT_SECRET>
		
```

### Options

```
      --as-machine             Initiates authentication as a machine via client credentials (client ID, client secret)
  -i, --client-id string       Client ID of the application.
  -s, --client-secret string   Client Secret of the application.
      --domain string          Specifies tenant domain when authenticating via client credentials (client ID, client secret)
  -h, --help                   help for login
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0](/auth0-cli/)	 - Supercharge your development workflow.


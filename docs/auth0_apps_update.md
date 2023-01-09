---
layout: default
---
# auth0 apps update

Update an application.

To update interactively, use `auth0 apps update` with no arguments.

To update non-interactively, supply the application id, name, type and other information you might want to change through the available flags.

## Usage
```
auth0 apps update [flags]
```

## Examples

```
  auth0 apps update
  auth0 apps update <app-id> --name myapp
  auth0 apps update <app-id> --name myapp --description <description>
  auth0 apps update <app-id> --name myapp --description <description> --type [native|spa|regular|m2m]
  auth0 apps update <app-id> --name myapp --description <description> --type [native|spa|regular|m2m] --reveal-client-secret
  auth0 apps update <app-id> -n myapp -d <description> -t [native|spa|regular|m2m] -r --json
```


## Flags

```
  -a, --auth-method string     Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).
  -c, --callbacks strings      After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native apps, all callbacks should use protocol https://.
  -d, --description string     Description of the application. Max character count is 140.
  -g, --grants strings         List of grant types supported for this application. Can include code, implicit, refresh-token, credentials, password, password-realm, mfa-oob, mfa-otp, mfa-recovery-code, and device-code.
      --json                   Output in json format.
  -l, --logout-urls strings    Comma-separated list of URLs that are valid to redirect to after logout from Auth0. Wildcards are allowed for subdomains.
  -n, --name string            Name of the application.
  -o, --origins strings        Comma-separated list of URLs allowed to make requests from JavaScript to Auth0 API (typically used with CORS). By default, all your callback URLs will be allowed. This field allows you to enter other origins if necessary. You can also use wildcards at the subdomain level (e.g., https://*.contoso.com). Query strings and hash information are not taken into account when validating these URLs.
  -r, --reveal-client-secret   Display the Client Secret as part of the command output.
  -t, --type string            Type of application:
                               - native: mobile, desktop, CLI and smart device apps running natively.
                               - spa (single page application): a JavaScript front-end app that uses an API.
                               - regular: Traditional web app using redirects.
                               - m2m (machine to machine): CLIs, daemons or services running on your backend.
  -w, --web-origins strings    Comma-separated list of allowed origins for use with Cross-Origin Authentication, Device Flow, and web message response mode.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 apps create](auth0_apps_create.md) - Create a new application
- [auth0 apps delete](auth0_apps_delete.md) - Delete an application
- [auth0 apps list](auth0_apps_list.md) - List your applications
- [auth0 apps open](auth0_apps_open.md) - Open the settings page of an application
- [auth0 apps show](auth0_apps_show.md) - Show an application
- [auth0 apps update](auth0_apps_update.md) - Update an application
- [auth0 apps use](auth0_apps_use.md) - Choose a default application for the Auth0 CLI



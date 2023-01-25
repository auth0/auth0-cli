---
layout: default
parent: auth0 email templates
has_toc: false
---
# auth0 email templates update

Update an email template.

To update interactively, use `auth0 email templates update` with no arguments.

To update non-interactively, supply the template name and other information through the flags.

## Usage
```
auth0 email templates update [flags]
```

## Examples

```
  auth0 email templates update
  auth0 email templates update <template>
  auth0 email templates update <template> --json
  auth0 email templates update welcome --enabled true
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100 --subject "Welcome"
  auth0 email templates update welcome --enabled true --body "$(cat path/to/body.html)" --from "welcome@example.com" --lifetime 6100 --subject "Welcome" --url "https://example.com"
  auth0 email templates update welcome -e true -b "$(cat path/to/body.html)" -f "welcome@example.com" -l 6100 -s "Welcome" -u "https://example.com" --json
```


## Flags

```
  -b, --body string      Body of the email template.
  -e, --enabled          Whether the template is enabled (true) or disabled (false). (default true)
      --force            Skip confirmation.
  -f, --from string      Sender's 'from' email address.
      --json             Output in json format.
  -l, --lifetime int     Lifetime in seconds that the link within the email will be valid for.
  -s, --subject string   Subject line of the email.
  -u, --url string       URL to redirect the user to after a successful action.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 email templates show](auth0_email_templates_show.md) - Show an email template
- [auth0 email templates update](auth0_email_templates_update.md) - Update an email template



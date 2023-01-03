---
layout: default
---
# auth0 email templates update

Update an email template.

To update interactively, use `auth0 email templates update` with no arguments.

To update non-interactively, supply the template name and other information through the flags.

```
auth0 email templates update [flags]
```


## Flags

```
  -b, --body string      Body of the email template.
  -e, --enabled          Whether the template is enabled (true) or disabled (false). (default true)
  -f, --from string      Sender's 'from' email address.
      --json             Output in json format.
  -l, --lifetime int     Lifetime in seconds that the link within the email will be valid for.
  -s, --subject string   Subject line of the email.
  -u, --url string       URL to redirect the user to after a successful action.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

## Examples

```
  auth0 email templates update <template>
  auth0 email templates update <template> --json
  auth0 email templates update welcome
```


## Related Commands

- [auth0 email templates show](auth0_email_templates_show.md) - Show an email template
- [auth0 email templates update](auth0_email_templates_update.md) - Update an email template



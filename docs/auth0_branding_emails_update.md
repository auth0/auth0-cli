---
layout: default
---
## auth0 branding emails update

Update an email template

### Synopsis

Update an email template.

```
auth0 branding emails update [flags]
```

### Examples

```
auth0 branding emails update <template>
auth0 branding emails update welcome
```

### Options

```
  -b, --body string      Body of the email template.
  -e, --enabled          Whether the template is enabled (true) or disabled (false). (default true)
  -f, --from string      Sender's 'from' email address.
  -h, --help             help for update
  -l, --lifetime int     Lifetime in seconds that the link within the email will be valid for.
  -s, --subject string   Subject line of the email.
  -u, --url string       URL to redirect the user to after a successful action.
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

* [auth0 branding emails](auth0_branding_emails.md)	 - Manage custom email templates


---
layout: default
---
## auth0 email templates update

Update an email template

### Synopsis

Update an email template.

```
auth0 email templates update [flags]
```

### Examples

```
auth0 email templates update <template>
auth0 email templates update welcome
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
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 email templates](auth0_email_templates.md)	 - Manage custom email templates

---
layout: default
parent: auth0 email provider
has_toc: false
---
# auth0 email provider update

Update the email provider.

To update interactively, use `auth0 email provider update` with no arguments.

To update non-interactively, supply the provider name and other information through the flags.

## Usage
```
auth0 email provider update [flags]
```

## Examples

```
  auth0 email provider update
  auth0 email provider update --json
  auth0 email provider update --enabled=false
  auth0 email provider update --credentials='{ "api_key":"NewAPIKey" }'
  auth0 email provider update --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --default-from-address="admin@example.com"
  auth0 email provider update --provider mandrill --enabled=true --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --provider mandrill --default-from-address='admin@example.com' --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --provider ses --credentials='{ "accessKeyId":"TheAccessKeyId", "secretAccessKey":"TheSecretAccessKey", "region":"eu" }' --settings='{ "message": { "configuration_set_name": "TheConfigurationSetName" } }'
  auth0 email provider update --provider sendgrid --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider update --provider sparkpost --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider update --provider sparkpost --credentials='{ "api_key":"TheAPIKey", "region":"eu" }'
  auth0 email provider update --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com"}'
  auth0 email provider update --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com", "region":"eu" }'
  auth0 email provider update --provider smtp --credentials='{ "smtp_host":"smtp.example.com", "smtp_port":25, "smtp_user":"smtp", "smtp_pass":"TheSMTPPassword" }'
  auth0 email provider update --provider azure_cs --credentials='{ "connection_string":"TheConnectionString" }'
  auth0 email provider update --provider ms365 --credentials='{ "tenantId":"TheTenantId", "clientId":"TheClientID", "clientSecret":"TheClientSecret" }'
  auth0 email provider update --provider custom --enabled=true --default-from-address="admin@example.com"
```


## Flags

```
  -c, --credentials string            Credentials for the email provider, formatted as JSON.
  -f, --default-from-address string   Provider default FROM address if none is specified.
  -e, --enabled                       Whether the provided is enabled (true) or disabled (false). (default true)
      --json                          Output in json format.
  -p, --provider string               Provider name. Can be 'mandrill', 'ses', 'sendgrid', 'sparkpost', 'mailgun', 'smtp', 'azure_cs', 'ms365', or 'custom'
  -s, --settings string               Settings for the email provider. formatted as JSON.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 email provider create](auth0_email_provider_create.md) - Create the email provider
- [auth0 email provider delete](auth0_email_provider_delete.md) - Delete the email provider
- [auth0 email provider show](auth0_email_provider_show.md) - Show the email provider
- [auth0 email provider update](auth0_email_provider_update.md) - Update the email provider



---
layout: default
parent: auth0 guardian factors sms
has_toc: false
---
# auth0 guardian factors sms show-templates

Display the SMS enrollment and verification message templates.

This is a legacy endpoint. Tenants on the unified phone experience must manage SMS templates from the Dashboard (Branding > Phone Templates); it is not available to Management API tokens.

## Usage
```
auth0 guardian factors sms show-templates [flags]
```

## Examples

```
  auth0 guardian factors sms show-templates --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 guardian factors sms set-provider](auth0_guardian_factors_sms_set-provider.md) - Set the SMS provider (legacy)
- [auth0 guardian factors sms set-templates](auth0_guardian_factors_sms_set-templates.md) - Set the SMS templates (legacy)
- [auth0 guardian factors sms set-twilio](auth0_guardian_factors_sms_set-twilio.md) - Set the SMS Twilio configuration (legacy)
- [auth0 guardian factors sms show-provider](auth0_guardian_factors_sms_show-provider.md) - Show the SMS provider (legacy)
- [auth0 guardian factors sms show-templates](auth0_guardian_factors_sms_show-templates.md) - Show the SMS templates (legacy)
- [auth0 guardian factors sms show-twilio](auth0_guardian_factors_sms_show-twilio.md) - Show the SMS Twilio configuration (legacy)



---
layout: default
parent: auth0 guardian factors phone
has_toc: false
---
# auth0 guardian factors phone set-twilio

Set the Twilio configuration for the phone MFA factor.

This is a legacy endpoint. Tenants on the unified phone experience must manage phone delivery from the Dashboard (Branding > Phone Provider); it is not available to Management API tokens.

## Usage
```
auth0 guardian factors phone set-twilio [flags]
```

## Examples

```
  auth0 guardian factors phone set-twilio --sid AC... --auth-token <token> --from "+14155550100"
  auth0 guardian factors phone set-twilio --sid AC... --auth-token <token> --messaging-service-sid MG...
```


## Flags

```
      --auth-token string              Twilio authentication token.
      --from string                    Twilio 'from' phone number.
      --json                           Output in json format.
      --json-compact                   Output in compact json format.
      --messaging-service-sid string   Twilio messaging service SID.
      --sid string                     Twilio account SID.
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

- [auth0 guardian factors phone set-message-types](auth0_guardian_factors_phone_set-message-types.md) - Set the phone message types
- [auth0 guardian factors phone set-provider](auth0_guardian_factors_phone_set-provider.md) - Set the phone provider (legacy)
- [auth0 guardian factors phone set-templates](auth0_guardian_factors_phone_set-templates.md) - Set the phone templates (legacy)
- [auth0 guardian factors phone set-twilio](auth0_guardian_factors_phone_set-twilio.md) - Set the phone Twilio configuration (legacy)
- [auth0 guardian factors phone show-message-types](auth0_guardian_factors_phone_show-message-types.md) - Show the phone message types
- [auth0 guardian factors phone show-provider](auth0_guardian_factors_phone_show-provider.md) - Show the phone provider (legacy)
- [auth0 guardian factors phone show-templates](auth0_guardian_factors_phone_show-templates.md) - Show the phone templates (legacy)
- [auth0 guardian factors phone show-twilio](auth0_guardian_factors_phone_show-twilio.md) - Show the phone Twilio configuration (legacy)



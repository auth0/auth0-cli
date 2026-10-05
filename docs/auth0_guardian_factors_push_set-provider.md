---
layout: default
parent: auth0 guardian factors push
has_toc: false
---
# auth0 guardian factors push set-provider

Set the push-notification MFA provider. One of: guardian, sns, direct.

## Usage
```
auth0 guardian factors push set-provider [flags]
```

## Examples

```
  auth0 guardian factors push set-provider --provider guardian
  auth0 guardian factors push set-provider --provider sns --json
```


## Flags

```
      --json              Output in json format.
      --json-compact      Output in compact json format.
  -p, --provider string   Provider to use for the factor.
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

- [auth0 guardian factors push set-apns](auth0_guardian_factors_push_set-apns.md) - Set the APNs configuration
- [auth0 guardian factors push set-fcm](auth0_guardian_factors_push_set-fcm.md) - Set the FCM (legacy) configuration
- [auth0 guardian factors push set-fcmv1](auth0_guardian_factors_push_set-fcmv1.md) - Set the FCM v1 configuration
- [auth0 guardian factors push set-provider](auth0_guardian_factors_push_set-provider.md) - Set the push-notification provider
- [auth0 guardian factors push set-sns](auth0_guardian_factors_push_set-sns.md) - Set the SNS configuration
- [auth0 guardian factors push show-apns](auth0_guardian_factors_push_show-apns.md) - Show the APNs configuration
- [auth0 guardian factors push show-provider](auth0_guardian_factors_push_show-provider.md) - Show the push-notification provider
- [auth0 guardian factors push show-sns](auth0_guardian_factors_push_show-sns.md) - Show the SNS configuration
- [auth0 guardian factors push update-apns](auth0_guardian_factors_push_update-apns.md) - Update the APNs configuration
- [auth0 guardian factors push update-fcm](auth0_guardian_factors_push_update-fcm.md) - Update the FCM (legacy) configuration
- [auth0 guardian factors push update-fcmv1](auth0_guardian_factors_push_update-fcmv1.md) - Update the FCM v1 configuration
- [auth0 guardian factors push update-sns](auth0_guardian_factors_push_update-sns.md) - Update the SNS configuration



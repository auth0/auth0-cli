---
layout: default
parent: auth0 universal-login
has_toc: false
---
# auth0 universal-login switch

Switch the rendering mode for Universal Login. Note that this requires a custom domain to be configured for the tenant.

## Usage
```
auth0 universal-login switch [flags]
```

## Examples

```
  auth0 universal-login switch
  auth0 universal-login switch --prompt login-id --screen login-id --rendering-mode standard
  auth0 ul switch --prompt login-id --screen login-id --rendering-mode advanced
  auth0 ul switch -p login-id -s login-id -r standard
```


## Flags

```
  -p, --prompt string           Name of the prompt to to switch or customize.
  -r, --rendering-mode string   standardMode is recommended for customizating consistent, branded experience for users.
                                Alternatively, advancedMode is recommended for full customization/granular control of the login experience and to integrate own component design system
                                
  -s, --screen string           Name of the screen to to switch or customize.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-login customize](auth0_universal-login_customize.md) - Customize the Universal Login experience for the standard or advanced mode
- [auth0 universal-login prompts](auth0_universal-login_prompts.md) - Manage custom text for prompts
- [auth0 universal-login show](auth0_universal-login_show.md) - Display the custom branding settings for Universal Login
- [auth0 universal-login switch](auth0_universal-login_switch.md) - Switch the rendering mode for Universal Login
- [auth0 universal-login templates](auth0_universal-login_templates.md) - Manage custom Universal Login templates
- [auth0 universal-login update](auth0_universal-login_update.md) - Update the custom branding settings for Universal Login



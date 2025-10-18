---
layout: default
parent: auth0 universal-login
has_toc: false
---
# auth0 universal-login customize


Customize your Universal Login Experience. Note that this requires a custom domain to be configured for the tenant. 

* Standard mode is recommended for creating a consistent, branded experience for users. Choosing Standard mode will open a webpage
within your browser where you can edit and preview your branding changes.For a comprehensive list of editable parameters and their values,
please visit the [Management API Documentation](https://auth0.com/docs/api/management/v2)

⚠️  DEPRECATION NOTICE: Advanced mode will be deprecated on 2026-04-18
   For future Advanced Customizations, use: auth0 acul config <command>
* Advanced mode is recommended for full customization/granular control of the login experience and to integrate your own component design system. 
Choosing Advanced mode will open the default terminal editor, with the rendering configs:

![storybook](settings.json)

Closing the terminal editor will save the settings to your tenant.

## Usage
```
auth0 universal-login customize [flags]
```

## Examples

```
  auth0 universal-login customize
  auth0 ul customize
```


## Flags

```
  -f, --file string             File to save the rendering configs to.
  -p, --prompt string           Name of the prompt to to switch or customize.
  -r, --rendering-mode string   standardMode is recommended for customizating consistent, branded experience for users.
                                Alternatively, advancedMode is recommended for full customization/granular control of the login experience and to integrate own component design system
                                
  -s, --screen string           Name of the screen to customize.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-login customize](auth0_universal-login_customize.md) - ⚠️ Customize Universal Login (Advanced mode DEPRECATED)
- [auth0 universal-login prompts](auth0_universal-login_prompts.md) - Manage custom text for prompts
- [auth0 universal-login show](auth0_universal-login_show.md) - Display the custom branding settings for Universal Login
- [auth0 universal-login switch](auth0_universal-login_switch.md) - ⚠️ Switch rendering mode (DEPRECATED)
- [auth0 universal-login templates](auth0_universal-login_templates.md) - Manage custom Universal Login templates
- [auth0 universal-login update](auth0_universal-login_update.md) - Update the custom branding settings for Universal Login



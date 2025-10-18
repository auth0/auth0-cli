---
layout: default
parent: auth0 universal-login
has_toc: false
---
# auth0 universal-login update

Update the custom branding settings for Universal Login.

To update the settings for Universal Login interactively, use `auth0 universal-login update` with no arguments.

To update the settings for Universal Login non-interactively, supply the accent, background and logo through the flags.

## Usage
```
auth0 universal-login update [flags]
```

## Examples

```
  auth0 universal-login update
  auth0 ul update --accent "#FF4F40" --background "#2A2E35" --logo "https://example.com/logo.png"
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png"
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png" --json
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png" --json-compact
```


## Flags

```
  -a, --accent string       Accent color.
  -b, --background string   Page background color
  -f, --favicon string      URL for the favicon. Must use HTTPS.
  -c, --font string         URL for the custom font. The URL must point to a font file and not a stylesheet. Must use HTTPS.
      --json                Output in json format.
      --json-compact        Output in compact json format.
  -l, --logo string         URL for the logo. Must use HTTPS.
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



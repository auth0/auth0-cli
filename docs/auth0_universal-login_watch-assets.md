---
layout: default
parent: auth0 universal-login
has_toc: false
---
# auth0 universal-login watch-assets



## Usage
```
auth0 universal-login watch-assets [flags]
```

## Examples

```
  auth0 universal-login watch-assets --screens login-id,login,signup,email-identifier-challenge,login-passwordless-email-code --watch-folder "/dist" --assets-url "http://localhost:8080"
  auth0 ul watch-assets --screens all -w "/dist" -u "http://localhost:8080"
  auth0 ul watch-assets --screen login-id --watch-folder "/dist"" --assets-url "http://localhost:8080"
  auth0 ul switch -p login-id -s login-id -r standard
```


## Flags

```
  -u, --assets-url string     Base URL for serving dist assets (e.g., http://localhost:5173).
  -s, --screens strings       watching screens
  -w, --watch-folder string   Folder to watch for new builds. CLI will watch for changes in the folder and automatically update the assets.
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
- [auth0 universal-login watch-assets](auth0_universal-login_watch-assets.md) - Watch dist folder and patch screen assets. We can watch for all or 1 or more screens.



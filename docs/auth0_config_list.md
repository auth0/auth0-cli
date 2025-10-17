---
layout: default
parent: auth0 config
has_toc: false
---
# auth0 config list

List Universal Login rendering configurations with optional filters and pagination.

## Usage
```
auth0 config list [flags]
```

## Examples

```
  auth0 acul config list --prompt login-id --screen login --rendering-mode advanced --include-fields true --fields head_tags,context_configuration
```


## Flags

```
      --fields string           Comma-separated list of fields to include or exclude in the result (based on value provided for include_fields) 
      --include-fields          Whether specified fields are to be included (default: true) or excluded (false). (default true)
      --include-totals          Return results inside an object that contains the total result count (true) or as a direct array of results (false).
      --json                    Output in json format.
      --json-compact            Output in compact json format.
      --page int                Page index of the results to return. First page is 0.
      --per-page int            Number of results per page. Default value is 50, maximum value is 100. (default 50)
      --prompt string           Filter by the Universal Login prompt.
  -q, --query string            Advanced query.
      --rendering-mode string   Filter by the rendering mode (advanced or standard).
      --screen string           Filter by the Universal Login screen.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 config docs](auth0_config_docs.md) - Open the ACUL configuration documentation
- [auth0 config generate](auth0_config_generate.md) - Generate a stub config file for a Universal Login screen.
- [auth0 config get](auth0_config_get.md) - Get the current rendering settings for a specific screen
- [auth0 config list](auth0_config_list.md) - List Universal Login rendering configurations
- [auth0 config set](auth0_config_set.md) - Set the rendering settings for a specific screen



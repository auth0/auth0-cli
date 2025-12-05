---
layout: default
parent: auth0 acul config
has_toc: false
---
# auth0 acul config list

List Universal Login rendering configurations with optional filters and pagination.

## Usage
```
auth0 acul config list [flags]
```

## Examples

```
  auth0 acul config list --prompt reset-password
  auth0 acul config list --rendering-mode advanced --include-fields true --fields head_tags,context_configuration
```


## Flags

```
      --fields string           Comma-separated list of fields to include or exclude in the result (based on value provided for include_fields) 
      --include-fields          Whether specified fields are to be included (true) or excluded (false). (default true)
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

- [auth0 acul config docs](auth0_acul_config_docs.md) - Open the ACUL configuration documentation
- [auth0 acul config generate](auth0_acul_config_generate.md) - Generate a stub config file for a Universal Login screen.
- [auth0 acul config get](auth0_acul_config_get.md) - Get the current rendering settings for a specific screen
- [auth0 acul config list](auth0_acul_config_list.md) - List Universal Login rendering configurations
- [auth0 acul config set](auth0_acul_config_set.md) - Set the rendering settings for a specific screen



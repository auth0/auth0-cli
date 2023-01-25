---
layout: default
parent: auth0 universal-login prompts
has_toc: false
---
# auth0 universal-login prompts show

Show the custom text for a prompt.

## Usage
```
auth0 universal-login prompts show [flags]
```

## Examples

```
  auth0 universal-login prompts show <prompt>
  auth0 universal-login prompts show <prompt> --language <language>
  auth0 ul prompts show <prompt> -l <language>
  auth0 ul prompts show signup -l es
```


## Flags

```
  -l, --language string   Language of the custom text. (default "en")
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-login prompts show](auth0_universal-login_prompts_show.md) - Show the custom text for a prompt
- [auth0 universal-login prompts update](auth0_universal-login_prompts_update.md) - Update the custom text for a prompt



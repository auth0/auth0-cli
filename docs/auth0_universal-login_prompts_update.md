---
layout: default
parent: auth0 universal-login prompts
has_toc: false
---
# auth0 universal-login prompts update

Update the custom text for a prompt.

## Usage
```
auth0 universal-login prompts update [flags]
```

## Examples

```
  auth0 universal-login prompts update <prompt>
  auth0 universal-login prompts update <prompt> --language <language>
  auth0 ul prompts update signup -l es

  # JSON input mode (for agents and automation)
  auth0 ul prompts update signup -l es --data '{"signup":{"title":"Sign Up"}}'
  auth0 ul prompts update signup -l es --data @signup.json
```


## Flags

```
      --data string       JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -l, --language string   Language of the custom text. (default "en")
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

- [auth0 universal-login prompts show](auth0_universal-login_prompts_show.md) - Show the custom text for a prompt
- [auth0 universal-login prompts update](auth0_universal-login_prompts_update.md) - Update the custom text for a prompt



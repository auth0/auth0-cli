---
layout: default
parent: auth0 agent skills
has_toc: false
---
# auth0 agent skills install

Install the Auth0 skill into your AI coding assistants via the pinned skills CLI (skills@1.5.23), run through npx (requires Node.js).

## Usage
```
auth0 agent skills install [flags]
```

## Examples

```
  # Choose assistants interactively
  auth0 agent skills install

  # Install into specific assistants (comma-separated or repeatable)
  auth0 agent skills install --agent claude-code,cursor
  auth0 agent skills install --agent claude-code --agent cursor

  # Install into every supported assistant
  auth0 agent skills install --agent all

  # Reinstall without prompting
  auth0 agent skills install --force
```


## Flags

```
      --agent strings   Assistant ID(s) to install into: comma-separated or repeatable, or 'all'. Defaults to prompting.
      --force           Reinstall without prompting (skills always fetches the latest).
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

- [auth0 agent skills install](auth0_agent_skills_install.md) - Install the Auth0 skill for your AI coding assistants



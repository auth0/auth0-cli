---
layout: default
parent: auth0 agent skills
has_toc: false
---
# auth0 agent skills install

Download the Auth0 skill and install it into your detected AI coding assistants.

With no flags it prompts for which assistants to set up. Use --agent to select them non-interactively.

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

  # Install into every detected assistant
  auth0 agent skills install --agent all

  # Re-download even if already up to date
  auth0 agent skills install --force
```


## Flags

```
      --agent strings   Assistant ID(s) to install into: comma-separated or repeatable, or 'all'. Defaults to prompting.
      --force           Re-download the skill even if it is already up to date.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 agent skills install](auth0_agent_skills_install.md) - Install the Auth0 skill for your AI coding assistants



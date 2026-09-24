---
layout: default
parent: auth0 docs
has_toc: false
---
# auth0 docs search

Search the official Auth0 documentation and print matching pages with their URLs.

## Usage
```
auth0 docs search [flags]
```

## Examples

```
  auth0 docs search browser
  auth0 docs search "custom domains"
  auth0 docs search "refresh token" --json
  auth0 docs search mfa --language ja
  auth0 docs search actions --open
  auth0 docs search rules --json-compact | jq '.[] | {title, url}'
```


## Flags

```
      --csv               Output in csv format.
      --json              Output in json format.
      --json-compact      Output in compact json format.
  -l, --language string   Documentation language to search. One of: en, fr, ja. (default "en")
      --open              Open a result in the browser. In an interactive terminal you pick which one; otherwise the top result opens. Not supported in agent mode.
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

- [auth0 docs search](auth0_docs_search.md) - Search the Auth0 documentation



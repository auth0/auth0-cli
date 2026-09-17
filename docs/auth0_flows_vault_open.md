---
layout: default
parent: auth0 flows vault
has_toc: false
---
# auth0 flows vault open

Open a Vault app's page in the Auth0 Dashboard. This opens the app's Vault page (for example AUTH0, JWT, HTTP, or SLACK), not a specific connection.

## Usage
```
auth0 flows vault open [flags]
```

## Examples

```
  auth0 flows vault open
  auth0 flows vault open HTTP
  auth0 flows vault open SLACK
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

- [auth0 flows vault connections](auth0_flows_vault_connections.md) - Manage Flow vault connections.
- [auth0 flows vault open](auth0_flows_vault_open.md) - Open the Vault in the Auth0 Dashboard



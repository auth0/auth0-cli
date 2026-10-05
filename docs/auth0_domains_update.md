---
layout: default
parent: auth0 domains
has_toc: false
---
# auth0 domains update

Update a custom domain.

To update interactively, use `auth0 domains update` with no arguments.

To update non-interactively, supply the domain name, type, policy and other information through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 domains update [flags]
```

## Examples

```
  auth0 domains update
  auth0 domains update <domain-id> --policy compatible
  auth0 domains update <domain-id> --policy compatible --ip-header "cf-connecting-ip"
  auth0 domains update <domain-id> --metadata '{"key1":"value1","key2":null}'
  auth0 domains update <domain-id> -p compatible -i "cf-connecting-ip" --json
  auth0 domains update <domain-id> -p compatible -i "cf-connecting-ip" --json-compact

  # Discover the payload schema
  auth0 domains update --schema
  auth0 domains update --schema --json

  # JSON input mode (for agents and automation)
  auth0 domains update <domain-id> --data '{"tls_policy":"recommended"}'
  auth0 domains update <domain-id> --data @domain.json
  cat domain.json | auth0 domains update <domain-id>
```


## Flags

```
      --data string        JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -i, --ip-header string   The HTTP header to fetch the client's IP address.
      --json               Output in json format.
      --json-compact       Output in compact json format.
  -m, --metadata string    The Custom Domain Metadata, formatted as JSON.
  -p, --policy string      The TLS version policy. Can be either 'compatible' or 'recommended'.
      --schema             Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 domains create](auth0_domains_create.md) - Create a custom domain
- [auth0 domains default](auth0_domains_default.md) - Manage the default custom domain
- [auth0 domains delete](auth0_domains_delete.md) - Delete a custom domain
- [auth0 domains list](auth0_domains_list.md) - List your custom domains
- [auth0 domains show](auth0_domains_show.md) - Show a custom domain
- [auth0 domains update](auth0_domains_update.md) - Update a custom domain
- [auth0 domains verify](auth0_domains_verify.md) - Verify a custom domain



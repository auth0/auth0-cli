---
layout: default
parent: auth0 tenant-settings update
has_toc: false
---
# auth0 tenant-settings update set

Enable selected tenant setting flags.

To enable interactively, use `auth0 tenant-settings update set` with no arguments.

To enable non-interactively, supply the flags.

To update tenant settings with a JSON payload, use `--data` or pipe the JSON. Run `auth0 tenant-settings update set --schema` to see the fields that the payload accepts.

The payload schema does not include these flags, so `--data` rejects them: flags.enable_custom_domain_in_emails, flags.enable_legacy_logs_search_v2, flags.universal_login and flags.use_scope_descriptions_for_consent. To change them, use the setting names with `auth0 tenant-settings update set` or `auth0 tenant-settings update unset`

## Usage
```
auth0 tenant-settings update set [flags]
```

## Examples

```
auth0 tenant-settings update set
auth0 tenant-settings update set <setting1> <setting2> <setting3>
auth0 tenant-settings update set flags.enable_client_connections mtls.enable_endpoint_aliases pushed_authorization_requests_supported
auth0 tenant-settings update set flags.enable_sso --json

# Discover the payload schema
auth0 tenant-settings update set --schema
auth0 tenant-settings update set --schema --json

# JSON input mode (for agents and automation)
auth0 tenant-settings update set --data '{"friendly_name":"Acme","support_email":"support@example.com"}'
auth0 tenant-settings update set --data @tenant-settings.json
cat tenant-settings.json | auth0 tenant-settings update set
```


## Flags

```
      --data string    JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --schema         Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 tenant-settings update set](auth0_tenant-settings_update_set.md) - Enable tenant setting flags
- [auth0 tenant-settings update unset](auth0_tenant-settings_update_unset.md) - Disable tenant setting flags



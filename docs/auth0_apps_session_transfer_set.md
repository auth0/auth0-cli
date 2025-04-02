---
layout: default
title: auth0 apps session-transfer set
parent: auth0 apps session-transfer
---

# auth0 apps session-transfer set

Set or update session transfer settings for an application.

## Usage

```bash
auth0 apps session-transfer set <app-id> [flags]
```

If no `<app-id>` is provided, the CLI will prompt you to select one.

## Description

Use this command to configure session transfer settings for a specific application. You can:

- Enable or disable token creation
- Restrict allowed authentication methods
- Enforce device binding policies

## Flags

| Flag                       | Description                                                                 |
|----------------------------|-----------------------------------------------------------------------------|
| `--can-create-token`       | Enable or disable creation of session transfer tokens (`true` or `false`)   |
| `--allowed-auth-methods`   | Comma-separated list: `cookie`, `query`                                     |
| `--enforce-device-binding` | Device binding mode: `none`, `ip`, or `asn`                                 |

> At least one flag must be provided or the command will return an error.

## Examples

```bash
# Enable session transfer with full configuration
auth0 apps session-transfer set <app-id> \
  --can-create-token=true \
  --allowed-auth-methods=cookie,query \
  --enforce-device-binding=ip

# Update only the allowed authentication methods
auth0 apps session-transfer set <app-id> \
  --allowed-auth-methods=cookie

# Disable device binding only
auth0 apps session-transfer set <app-id> \
  --enforce-device-binding=none

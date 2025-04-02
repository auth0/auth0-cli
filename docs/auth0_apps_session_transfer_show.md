---
layout: default
title: auth0 apps session-transfer show
parent: auth0 apps session-transfer
---

# auth0 apps session-transfer show

Show session transfer settings for an application.

## Usage

```bash
auth0 apps session-transfer show <app-id> [flags]
```

If no `<app-id>` is provided, the CLI will prompt you to select one.

## Description

Displays the session transfer configuration for the specified application. The output includes:

- The application's `Client ID` and `Name`
- Whether session transfer tokens can be created
- Allowed authentication methods (`cookie`, `query`)
- Device binding enforcement policy (`none`, `ip`, or `asn`)

## Flags

| Name     | Description               |
|----------|---------------------------|
| `--json` | Output in JSON format     |

## Example

```bash
auth0 apps session-transfer show <app-id>

# Migration Guide

## Exit codes and error output

Process exit codes stay coarse and backwards compatible: `0` on success and `1` on any failure, matching the CLI's long-standing behavior. The one addition is `130`, which is now returned when a command is interrupted with `Ctrl-C` (previously `0`), so an interrupted run reports failure.

| Exit code | Meaning |
| --------- | ------- |
| `0` | Success |
| `1` | Any failure |
| `130` | Interrupted with `Ctrl-C` (previously `0`) |

The granular failure class is not carried by the exit code. Instead, when running with `--json` or in agent mode, errors are written to stderr as a single-line JSON envelope so they can be parsed programmatically:

```json
{"error":{"code":"not_found","message":"...","status":404}}
```

The `code` field classifies the failure (`usage`, `auth`, `validation`, `not_found`, `rate_limit`, `api`, or `unknown`), so agents and scripts can branch on the class without parsing human text or relying on a specific exit code. The `status` field carries the HTTP status when the error came from the Auth0 Management API, and the `details` field carries field-level validation errors when the API returns them. Human-readable error output is unchanged in normal (non-JSON, non-agent) mode.

Automation that only distinguishes success (`0`) from failure (non-zero) is unaffected.

## Agent mode output

Agent mode makes the whole output stream machine-readable. It is enabled automatically when the CLI detects an AI agent, and you can force it with `--agent-mode` (or `AUTH0_AGENT_MODE=true`) or turn it off with `--agent-mode=false` (or `AUTH0_AGENT_MODE=false`). The behavior below applies in agent mode, and the streaming and JSON-newline behavior also applies whenever `--json` is used.

- Streaming commands emit newline-delimited JSON (one JSON object per line) instead of a JSON array. For example `auth0 logs tail --json` previously rendered a table and now prints one log event per line, so a reader can consume events incrementally without waiting for an array that never closes while tailing.
- JSON results on stdout always end with a trailing newline, so a piped or newline-delimited reader never drops the final record.
- Diagnostic messages (info, success, detail, warning, and non-fatal errors) are written to stderr as JSON lines in the form `{"level":"...","message":"..."}` instead of decorated human prose, matching the JSON error envelope above. The decorative heading is suppressed.
- Help is returned as JSON. A bare `auth0` and a command group invoked without a subcommand (for example `auth0 apps`) return the JSON command tree rather than the human help text, and the root help describes agent mode itself.

If you parse the human-readable prose that the CLI previously printed to stderr, or you depend on streaming commands emitting a single JSON array, update your automation to read the JSON lines and newline-delimited output described here. Normal (non-JSON, non-agent) mode is unchanged.

## Upgrading from v0.x → v1.0

As is to be expected with a major release, there are breaking changes in this update. Please ensure you read this guide
thoroughly and prepare your potential automated workflows before upgrading to the Auth0 CLI v1.

### Breaking Changes

- [Commands Reorganization](#commands-reorganization)
- [Authenticating With Client Credentials](#authenticating-with-client-credentials)
- [`test` Commands](#test-commands)
- [JSON Output Flag](#json-output-flag)
- [Reveal Client Secrets Flag](#reveal-client-secrets-flag)
- [Config Command Removal](#config-command-removal)
- [Users Commands](#users-commands)

#### Commands Reorganization

Some commands have been reorganized to establish a more systematic hierarchy.
All other facets of the commands (arguments, flags, etc.) remain the same.

| **Before (v0)**            | **After (v1)**                                  |
| -------------------------- | ----------------------------------------------- |
| `auth0 ips`                | `auth0 protection suspicious-ip-throttling ips` |
| `auth0 users unblock`      | `auth0 users blocks unblock`                    |
| `auth0 branding domains`   | `auth0 domains`                                 |
| `auth0 branding emails`    | `auth0 email templates`                         |
| `auth0 branding show`      | `auth0 universal-login show`                    |
| `auth0 branding update`    | `auth0 universal-login update`                  |
| `auth0 branding templates` | `auth0 universal-login templates`               |
| `auth0 branding texts`     | `auth0 universal-login prompts`                 |

#### Authenticating With Client Credentials

The `auth0 tenants add` command which enabled authenticating to a tenant via client credentials has been consolidated
into the `auth0 login` command. It can be interfaced interactively through the terminal or non-interactively by passing
in the client credentials through the flags.

<table>
<tr>
<th>Before (v0)</th>
<th>After (v1)</th>
</tr>
<tr>
<td>

```sh
# Example:
auth0 tenants add travel0.us.auth0.com \
--client-id tUIvAH7g2ykVM4lGriYEQ6BKV3je24Ka \
--client-secret XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

</td>
<td>

```sh
# Example:
auth0 login --domain travel0.us.auth0.com \
--client-id tUIvAH7g2ykVM4lGriYEQ6BKV3je24Ka \
--client-secret XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

</td>
</tr>
</table>

#### `test` Commands

The `auth0 test token` and `auth0 test login` commands have been adjusted to facilitate a better developer experience. Specifically, the `--connection` flag has been renamed to `--connection-name` for clarity, the `--client-id` flag changed into an argument. Also noteworthy is that a test application is no longer created by default when running `auth0 test token` without a provided client but still presented as an option.

| **Before (v0)**                 | **After (v1)**                       |
| ------------------------------- | ------------------------------------ |
| `auth0 test login --connection` | `auth0 test login --connection-name` |
| `auth0 test token --client-id`  | `auth0 test token <client-id>`       |

#### JSON Output Flag

The `--format json` flag-value pair has been condensed into the `--json` flag.

| **Before (v0)**                 | **After (v1)**           |
| ------------------------------- | ------------------------ |
| `auth0 apps list --format json` | `auth0 apps list --json` |

#### Log Streams

In `v0.x`, the creation and updating of log streams through the `auth0 logs streams create` and
`auth0 log streams update` commands facilitated the management of all log stream types with a mass of
type-specific flags. For `v1.x`, the type of log stream is now required as an argument.
This change facilitates more ergonomic flags and type-specific validations.

<table>
<tr>
<th>Before (v0)</th>
<th>After (v1)</th>
</tr>
<tr>
<td>

```sh
# Example:
auth0 logs streams create \
--type datadog \
--name "My Datadog Log Stream" \
--datadog-id us \
--datadog-key 3c0c4965368b6b10f8640dbda46abfdc
```

</td>
<td>

```sh
# Example:
auth0 logs streams create datadog \
--name "My Datadog Log Stream" \
--region us \
--api-key 3c0c4965368b6b10f8640dbda46abfdc
```

</td>
</tr>
</table>

#### Reveal Client Secrets Flag

In `v0.x`, the `auth0 apps create` command has a `--reveal` flag that would reveal the client secrets in the output.
This flag has changed to `--reveal-secrets` to clarify what is being revealed.

| **Before (v0)**              | **After (v1)**                       |
| ---------------------------- | ------------------------------------ |
| `auth0 apps create --reveal` | `auth0 apps create --reveal-secrets` |

#### Config Command Removal

In `v0.x`, the undocumented `auth0 config init` command existed to authenticate with a tenant for E2E testing.
It authenticated with tenants via client credentials which were sourced from environment variables.
This command has been removed in favor of the `auth0 login` command.

<table>
<tr>
<th>Before (v0)</th>
<th>After (v1)</th>
</tr>
<tr>
<td>

```sh
# Example:
AUTH0_CLI_CLIENT_DOMAIN="travel0.us.auth0.com" \
AUTH0_CLI_CLIENT_ID="tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka" \
AUTH0_CLI_CLIENT_SECRET="XXXXXXXXXXXXXXXXXXXXXXXXXXXX" \
auth0 config init
```

</td>
<td>

```sh
# Example:
auth0 login --domain travel0.us.auth0.com \
--client-id tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka \
--client-secret XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

</td>
</tr>
</table>

#### Users Commands

The `--connection` flag has been renamed to `--connection-name` for the `auth0 users update`, `auth0 users create` and `auth0 users import` commands for consistency.

Also notably for the `auth0 users import` command, the `-u` short form alias for the `--upsert` flag in the command has been redesignated to the `--users` flag.

| **Before (v0)**                      | **After (v1)**                                  |
| ------------------------------------ | ----------------------------------------------- |
| `auth0 users create --connection`    | `auth0 users create --connection-name`          |
| `auth0 users update --connection`    | `auth0 users update --connection-name`          |
| `auth0 users import -u --connection` | `auth0 users import --upsert --connection-name` |

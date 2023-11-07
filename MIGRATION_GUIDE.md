# Migration Guide

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
AUTH0_DOMAIN="travel0.us.auth0.com" \
AUTH0_CLIENT_ID="tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka" \
AUTH0_CLIENT_SECRET="XXXXXXXXXXXXXXXXXXXXXXXXXXXX" \
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

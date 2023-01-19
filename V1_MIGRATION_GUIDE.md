# V1 Migration Guide

Guide to migrating from `v0.x` to `v1.x` of the Auth0 CLI.

## Branding Commands Reorganization

The branding commands have been reorganized to establish a more systematic hierarchy:

| **Before (v0)**            | **After (v1)**                    |
| -------------------------- | --------------------------------- |
| `auth0 branding domains`   | `auth0 domains`                   |
| `auth0 branding emails`    | `auth0 email templates`           |
| `auth0 branding show`      | `auth0 universal-login show`      |
| `auth0 branding update`    | `auth0 universal-login update`    |
| `auth0 branding templates` | `auth0 universal-login templates` |
| `auth0 branding texts`     | `auth0 universal-login prompts`   |

## JSON Output Flag

The `--format json` flag-value pair has been condensed into the `--json` flag.

**Before:** `auth0 apps list --format json`

**After:** `auth0 apps list --json`

## Authenticating With Client Credentials

The `auth0 tenants add` command which enabled authenticating to a tenant via client credentials has been consolidated into the `auth0 login` command. It can be interfaced interactively through the terminal or non-interactively by passing in the client credentials with flags (example below).

**Before:**

```sh
auth0 tenants add travel0.us.auth0.com \
--client-id tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka \
--client-secret zvKP0uWfsF0mBEgAOCgAMdFXgthfwgt_GXf9eEeMOWEIOlPK8pc3b119qBL0b2av
```

**After:**

```sh
auth0 login --domain travel0.us.auth0.com \
--client-id tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka \
--client-secret zvKP0uWfsF0mBEgAOCgAMdFXgthfwgt_GXf9eEeMOWEIOlPK8pc3b119qBL0b2av
```

## Log Streams

In `v0.x`, the creation and updating of log streams through the `auth0 logs streams create` and `auth0 log streams update` commands facilitated the management of all log stream types with a mass of type-specific flags. For `v1.x`, the type of log stream is now required as an argument. This change facilitates more ergonomic flags and type-specific validations.

**Before:**

```sh
auth0 logs streams create \
--type datadog \
--name "My Datadog Log Stream" \
--datadog-id us \
--datadog-key 3c0c4965368b6b10f8640dbda46abfdc
```

**After:**

```sh
auth0 logs streams create datadog \
--name "My Datadog Log Stream" \
--region us \
--api-key 3c0c4965368b6b10f8640dbda46abfdc
```

## Reveal Client Secret Flag

In `v0.x`, the `auth0 apps create` command has a `--reveal` flag that would unobscure the client secrets from the output. This flag has changed to `--reveal-client-secret` for clarify what is being revealed.

**Before:** `auth0 apps create --reveal`

**After:** `auth0 apps create --reveal-client-secret`

## Config Command Removal

In `v0.x`, the undocumented `auth0 config init` command existed to authenticate with a tenant for E2E testing. It authenticated with tenants via client credentials which were sourced from environment variables. This command has been removed in favor of the `auth0 login` command.

**Before:**

```
AUTH0_CLI_CLIENT_DOMAIN="travel0.us.auth0.com"\
AUTH0_CLI_CLIENT_ID="tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka"\
AUTH0_CLI_CLIENT_SECRET="zvKP0uWfsF0mBEgAOCgAMdFXgthfwgt_GXf9eEeMOWEIOlPK8pc3b119qBL0b2av"\
auth0 config init
```

**After:**

```
auth0 login --domain travel0.us.auth0.com \
--client-id tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka \
--client-secret zvKP0uWfsF0mBEgAOCgAMdFXgthfwgt_GXf9eEeMOWEIOlPK8pc3b119qBL0b2av
```

# V1 Migration Guide

Guide to migrating from `0.x` to `1.x`

## Branding Commands Reorganization

The branding commands have been reorganized to establish a more systematic hierarchy:

| **v0.x**                   | **v1.x**                          |
| -------------------------- | --------------------------------- |
| `auth0 branding templates` | `auth0 universal-login templates` |
| `auth0 branding emails`    | `auth0 email templates`           |
| `auth0 branding domains`   | `auth0 domains`                   |
| `auth0 branding texts`     | `auth0 universal-login prompts`   |
| `auth0 branding show`      | `auth0 universal-login show`      |
| `auth0 branding update`    | `auth0 universal-login update`    |

## JSON Output Flag

The `--format` flag with the single `json` argument has been condensed into the `--json` flag.

Additionally, the JSON output flag has been relegated from the global flag list and only registered for commands where applicable.

## Reveal Client Secret Flag

In `v0.x` the `auth0 apps create` command has a `--reveal` flag that would unobscure the client secrets from the output. This flag has changed to `--reveal-client-secret` for clarity.

## Authenticating with Client Credentials

The `auth0 tenants add` command which enabled authenticating to a tenant via client credentials has been consolidated into the `auth0 login` command. It can be interfaced interactively through the terminal or non-interactively by passing in the client credentials with flags (example below).

```sh
auth0 login --domain travel0.us.auth0.com \
--client-id tUIvPH7g2ykVm4lGriYEQ6BKV3je24Ka \
--client-secret zvKP0uWfsF0mBEgAOCgAMdFXgthfwgt_GXf9eEeMOWEIOlPK8pc3b119qBL0b2av
```

## Log Streams

In `v0.x` the `auth0 logs streams` comma

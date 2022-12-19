<div align="center">
  <h1>Auth0 CLI</h1>

[![Release](https://img.shields.io/github/v/release/auth0/auth0-cli?include_prereleases&style=flat-square)](https://github.com/auth0/auth0-cli/releases) [![Build Status](https://img.shields.io/github/actions/workflow/status/auth0/auth0-cli/go.yml?branch=main)](https://github.com/auth0/auth0-cli/actions?query=branch%3Amain) [![Go Report Card](https://goreportcard.com/badge/github.com/auth0/auth0-cli?style=flat-square)](https://goreportcard.com/report/github.com/auth0/auth0-cli) [![License](https://img.shields.io/github/license/auth0/auth0-cli.svg?style=flat-square)](https://github.com/auth0/auth0-cli/blob/main/LICENSE)

</div>

# Auth0 CLI

Build, manage and test your [Auth0](http://auth0.com/) integrations from the command line.

![demo](./demo.gif)

---

## Highlights

- **🧪 Test your universal login flow:** Emulate your end users' login experience by running `auth0 test login`.
- **🔍 Troubleshoot in real-time:** Inspect the events of your Auth0 integration as they happen with the `auth0 logs tail` command
- **🔁 Simplify repetitive tasks:** Create, update, list and delete your Auth0 resources directly from the terminal.

---

## Table of Contents

- [Installation](#installation)
- [Authentication](#authentication)
- [Available Commands](#available-commands)
- [Customization](#customization)
- [Anonymous Analytics](#anonymous-analytics)

---

## Installation

### macOS

Install via [Homebrew](https://brew.sh/):

```bash
 brew tap auth0/auth0-cli && brew install auth0
```

### Windows

Install via [Scoop](https://scoop.sh/):

```bash
scoop bucket add auth0 https://github.com/auth0/scoop-auth0-cli.git
scoop install auth0
```

### Linux

Install via [cURL](https://curl.se/):

```bash
# Binary will be downloaded to "./auth0".
curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/main/install.sh | sh -s -- -b .
```

### Manual

1. Download the appropriate binary for your environment from the [latest release](https://github.com/auth0/auth0-cli/releases/latest/)
2. Extract the archive
   - **macOS**: `$ tar -xf auth0-cli_{version}_Darwin_{architecture}.tar.gz`
   - **Linux**: `$ tar -xf auth0-cli_{version}_Linux_{architecture}.tar.gz`
   - **Windows**: Extract `auth0-cli_{version}_Windows_{architecture}.zip` using your preferred method of choice
3. Run `./auth0`

> **Note**
> Autocompletion instructions for supported platforms available by running `auth0 completion -h`

### Go

Install via [Go](https://go.dev/):

```bash
go install github.com/auth0/auth0-cli/cmd/auth0@latest
```

## Authentication

Authenticating to your Auth0 tenant is required for most functions of the CLI. It can be initiated by running:

```bash
auth0 login
```

There are two ways to authenticate:

- **As a user** - Recommended when invoking on a personal machine or other interactive environment. Facilitated by [device authorization](https://auth0.com/docs/get-started/authentication-and-authorization-flow/device-authorization-flow) flow.
- **As a machine** - Recommended when running on a server or non-interactive environments (ex: CI). Facilitated by [client credentials](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow) flow. Flags available for bypassing interactive shell.

> **Warning**
> Authenticating as a user is not supported for **private cloud** tenants. Instead, those users should authenticate with client credentials.

## Available Commands

- [auth0 actions](auth0_actions.md) - Manage resources for actions
- [auth0 api](auth0_api.md) - Makes an authenticated HTTP request to the Auth0 Management API
- [auth0 apis](auth0_apis.md) - Manage resources for APIs
- [auth0 apps](auth0_apps.md) - Manage resources for applications
- [auth0 completion](auth0_completion.md) - Setup autocomplete features for this CLI on your terminal
- [auth0 domains](auth0_domains.md) - Manage custom domains
- [auth0 email](auth0_email.md) - Manage email settings
- [auth0 ips](auth0_ips.md) - Manage blocked IP addresses
- [auth0 login](auth0_login.md) - Authenticate the Auth0 CLI
- [auth0 logout](auth0_logout.md) - Log out of a tenant's session
- [auth0 logs](auth0_logs.md) - View tenant logs
- [auth0 orgs](auth0_orgs.md) - Manage resources for organizations
- [auth0 protection](auth0_protection.md) - Manage resources for attack protection
- [auth0 quickstarts](auth0_quickstarts.md) - Quickstart support for getting bootstrapped
- [auth0 roles](auth0_roles.md) - Manage resources for roles
- [auth0 rules](auth0_rules.md) - Manage resources for rules
- [auth0 tenants](auth0_tenants.md) - Manage configured tenants
- [auth0 test](auth0_test.md) - Try your Universal Login box or get a token
- [auth0 universal-login](auth0_universal-login.md) - Manage the Universal Login experience
- [auth0 users](auth0_users.md) - Manage resources for users

## Customization

To change the text editor used for editing templates, rules, and actions,
set the environment variable `EDITOR` to your preferred editor. Example:

```shell
export EDITOR="code -w"
# or
export EDITOR="nano"
```

## Anonymized Analytics Disclosure

Anonymized data points are collected during the use of this CLI. This data includes the CLI version, operating system, timestamp, and other technical details that do not personally identify you.

Auth0 uses this data to better understand the usage of this tool to prioritize the features, enhancements and fixes that matter most to our users.

To **opt-out** of this collection, set the `AUTH0_CLI_ANALYTICS` environment variable to `false`.

## Issue Reporting

For general support or usage questions, use the [Auth0 Community](https://community.auth0.com/) forums or raise a [support ticket](https://support.auth0.com/). Only [raise an issue](https://github.com/auth0/auth0-cli/issues) if you have found a bug or want to request a feature.

**Do not report security vulnerabilities on the public GitHub issue tracker.** The [Responsible Disclosure Program](https://auth0.com/responsible-disclosure-policy) details the procedure for disclosing security issues.

## What is Auth0?

Auth0 helps you to:

- Add authentication with [multiple sources](https://auth0.com/docs/authenticate/identity-providers), either social identity providers such as **Google, Facebook, Microsoft Account, LinkedIn, GitHub, Twitter, Box, Salesforce** (amongst others), or enterprise identity systems like **Windows Azure AD, Google Apps, Active Directory, ADFS, or any SAML identity provider**.
- Add authentication through more traditional **[username/password databases](https://auth0.com/docs/authenticate/database-connections/custom-db)**.
- Add support for **[linking different user accounts](https://auth0.com/docs/manage-users/user-accounts/user-account-linking)** with the same user.
- Support for generating signed [JSON Web Tokens](https://auth0.com/docs/secure/tokens/json-web-tokens) to call your APIs and **flow the user identity** securely.
- Analytics of how, when, and where users are logging in.
- Pull data from other sources and add it to the user profile through [JavaScript Actions](https://auth0.com/docs/customize/actions).

**Why Auth0?** Because you should save time, be happy, and focus on what really matters: building your product.

## License

This project is licensed under the MIT license. See the [LICENSE](LICENSE) file for more information.

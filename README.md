<div align="center">
  <h1>Auth0 CLI</h1>

[![Release](https://img.shields.io/github/v/release/auth0/auth0-cli?include_prereleases&style=flat-square)](https://github.com/auth0/auth0-cli/releases) [![Build Status](https://img.shields.io/github/actions/workflow/status/auth0/auth0-cli/go.yml?branch=main)](https://github.com/auth0/auth0-cli/actions?query=branch%3Amain) [![Go Report Card](https://goreportcard.com/badge/github.com/auth0/auth0-cli?style=flat-square)](https://goreportcard.com/report/github.com/auth0/auth0-cli) [![License](https://img.shields.io/github/license/auth0/auth0-cli.svg?style=flat-square)](https://github.com/auth0/auth0-cli/blob/main/LICENSE)

</div>

Build, manage and test your [Auth0](https://auth0.com/) integrations from the command line.

![demo](./demo.gif)

## Highlights

- **🧪 Test your universal login flow:** Emulate your end users' login experience by running `auth0 test login`.
- **🔍 Troubleshoot in real-time:** Inspect the events of your Auth0 integration as they happen with the `auth0 logs tail` command.
- **🔁 Simplify repetitive tasks:** Create, update, list and delete your Auth0 resources directly from the terminal.

## Table of Contents

- [Installation](#installation)
- [Authenticating to Your Tenant](#authenticating-to-your-tenant)
- [Available Commands](#available-commands)
- [Customization](#customization)
- [Anonymous Analytics](#anonymous-analytics)

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

## Authenticating to Your Tenant

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

- [auth0 actions](https://auth0.github.io/auth0-cli/auth0_actions.html) - Manage resources for actions
- [auth0 api](https://auth0.github.io/auth0-cli/auth0_api.html) - Makes an authenticated HTTP request to the Auth0 Management API
- [auth0 apis](https://auth0.github.io/auth0-cli/auth0_apis.html) - Manage resources for APIs
- [auth0 apps](https://auth0.github.io/auth0-cli/auth0_apps.html) - Manage resources for applications
- [auth0 completion](https://auth0.github.io/auth0-cli/auth0_completion.html) - Setup autocomplete features for this CLI on your terminal
- [auth0 domains](https://auth0.github.io/auth0-cli/auth0_domains.html) - Manage custom domains
- [auth0 email](https://auth0.github.io/auth0-cli/auth0_email.html) - Manage email settings
- [auth0 ips](https://auth0.github.io/auth0-cli/auth0_ips.html) - Manage blocked IP addresses
- [auth0 login](https://auth0.github.io/auth0-cli/auth0_login.html) - Authenticate the Auth0 CLI
- [auth0 logout](https://auth0.github.io/auth0-cli/auth0_logout.html) - Log out of a tenant's session
- [auth0 logs](https://auth0.github.io/auth0-cli/auth0_logs.html) - View tenant logs
- [auth0 orgs](https://auth0.github.io/auth0-cli/auth0_orgs.html) - Manage resources for organizations
- [auth0 protection](https://auth0.github.io/auth0-cli/auth0_protection.html) - Manage resources for attack protection
- [auth0 quickstarts](https://auth0.github.io/auth0-cli/auth0_quickstarts.html) - Quickstart support for getting bootstrapped
- [auth0 roles](https://auth0.github.io/auth0-cli/auth0_roles.html) - Manage resources for roles
- [auth0 rules](https://auth0.github.io/auth0-cli/auth0_rules.html) - Manage resources for rules
- [auth0 tenants](https://auth0.github.io/auth0-cli/auth0_tenants.html) - Manage configured tenants
- [auth0 test](https://auth0.github.io/auth0-cli/auth0_test.html) - Try your Universal Login box or get a token
- [auth0 universal-login](https://auth0.github.io/auth0-cli/auth0_universal-login.html) - Manage the Universal Login experience
- [auth0 users](https://auth0.github.io/auth0-cli/auth0_users.html) - Manage resources for users

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

## Feedback

### Contributing

We appreciate feedback and contribution to this repo! Before you get started, please see the following:

- [Auth0's general contribution guidelines](https://github.com/auth0/open-source-template/blob/master/GENERAL-CONTRIBUTING.md)
- [Auth0's code of conduct guidelines](https://github.com/auth0/open-source-template/blob/master/CODE-OF-CONDUCT.md)
- [This repo's contribution guide](https://github.com/auth0/auth0-cli/blob/main/CONTRIBUTING.md)

### Raise an issue

To provide feedback or report a bug, please [raise an issue on our issue tracker](https://github.com/auth0/auth0-cli/issues).

### Vulnerability Reporting

Please do not report security vulnerabilities on the public GitHub issue tracker. The [Responsible Disclosure Program](https://auth0.com/responsible-disclosure-policy) details the procedure for disclosing security issues.

---

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset="https://cdn.auth0.com/website/sdks/logos/auth0_light_mode.png"   width="150">
    <source media="(prefers-color-scheme: dark)" srcset="https://cdn.auth0.com/website/sdks/logos//auth0_dark_mode.png" width="150">
    <img alt="Auth0 Logo" src="https://cdn.auth0.com/website/sdks/logos/auth0_light_mode.png" width="150">
  </picture>
</p>
<p align="center">Auth0 is an easy to implement, adaptable authentication and authorization platform. To learn more checkout <a href="https://auth0.com/why-auth0">Why Auth0?</a></p>
<p align="center">
This project is licensed under the MIT license. See the <a href="https://github.com/auth0/auth0-cli/blob/main/LICENSE"> LICENSE</a> file for more info.</p>

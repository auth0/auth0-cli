---
layout: home
---

Build, manage and test your [Auth0](https://auth0.com/) integrations from the command line.

## Installation

### macOS

Install via [Homebrew](https://brew.sh/):

```
brew tap auth0/auth0-cli && brew install auth0
```

### Windows

Install via [Scoop](https://scoop.sh/):

```
scoop bucket add auth0 https://github.com/auth0/scoop-auth0-cli.git && scoop install auth0
```

Install via [Powershell](https://learn.microsoft.com/en-us/powershell/)

```
#fetch latest release information
$apiUrl = "https://api.github.com/repos/auth0/auth0-cli/releases/latest"
$latestRelease = Invoke-RestMethod -Uri $apiUrl
$latestVersion = $latestRelease.tag_name
$version = $latestVersion -replace "^v"
# Download the binary to the current folder
Invoke-WebRequest "https://github.com/auth0/auth0-cli/releases/download/${latestVersion}/auth0-cli_${version}_Windows_x86_64.zip" -OutFile ".\auth0.zip"
Expand-Archive ".\auth0.zip" .\
# To be able to run the binary from any directory, make sure you add it to your '$PATH'
[System.Environment]::SetEnvironmentVariable('PATH', $Env:PATH + ";${pwd}")
```

### Linux

Install via [cURL](https://curl.se/):

```
curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/main/install.sh | sh -s -- -b .
```

### Go

Install via [Go](https://go.dev/):

```
go install github.com/auth0/auth0-cli/cmd/auth0@latest
```

### Manual

1. Download the appropriate binary for your environment from the [latest release](https://github.com/auth0/auth0-cli/releases/latest/)
2. Extract the archive
3. Run `./auth0`

Autocompletion instructions for supported platforms available by running `auth0 completion -h`

## Authenticating to Your Tenant

Authenticating to your Auth0 tenant is required for most functions of the CLI. It can be initiated by running:

```
auth0 login
```

There are two ways to authenticate:

- **As a user** - Recommended when invoking on a personal machine or other interactive environment. Facilitated by [device authorization](https://auth0.com/docs/get-started/authentication-and-authorization-flow/device-authorization-flow) flow.
- **As a machine** - Recommended when running on a server or non-interactive environments (ex: CI). Facilitated by [client credentials](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow) flow. Flags available for bypassing interactive shell.

{: .warning }
Authenticating as a user is not supported for **private cloud** tenants. Instead, those users should authenticate with client credentials.

## Available Commands

- [auth0 actions](auth0_actions.md) - Manage resources for actions
- [auth0 acul](auth0_acul.md) - Advanced Customization the Universal Login experience
- [auth0 api](auth0_api.md) - Makes an authenticated HTTP request to the Auth0 Management API
- [auth0 apis](auth0_apis.md) - Manage resources for APIs
- [auth0 apps](auth0_apps.md) - Manage resources for applications
- [auth0 completion](auth0_completion.md) - Setup autocomplete features for this CLI on your terminal
- [auth0 domains](auth0_domains.md) - Manage custom domains
- [auth0 email](auth0_email.md) - Manage email settings and configure email providers
- [auth0 event-streams](auth0_event-streams.md) - Manage Event Stream
- [auth0 login](auth0_login.md) - Authenticate the Auth0 CLI
- [auth0 logout](auth0_logout.md) - Log out of a tenant's session
- [auth0 logs](auth0_logs.md) - View tenant logs
- [auth0 network-acl](auth0_network-acl.md) - Manage network ACL settings
- [auth0 orgs](auth0_orgs.md) - Manage resources for organizations
- [auth0 phone](auth0_phone.md) - Manage phone providers
- [auth0 protection](auth0_protection.md) - Manage resources for attack protection
- [auth0 quickstarts](auth0_quickstarts.md) - Quickstart support for getting bootstrapped
- [auth0 roles](auth0_roles.md) - Manage resources for roles
- [auth0 rules](auth0_rules.md) - Manage resources for rules
- [auth0 tenant-settings](auth0_tenant-settings.md) - Manage tenant settings
- [auth0 tenants](auth0_tenants.md) - Manage configured tenants
- [auth0 terraform](auth0_terraform.md) - Manage terraform configuration for your Auth0 Tenant
- [auth0 test](auth0_test.md) - Try your Universal Login box or get a token
- [auth0 token-exchange](auth0_token-exchange.md) - Manage token exchange profiles
- [auth0 universal-login](auth0_universal-login.md) - Manage the Universal Login experience
- [auth0 users](auth0_users.md) - Manage resources for users


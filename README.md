# Auth0 CLI (Experimental)

`auth0` is the command line to supercharge your development workflow.

> Note: This CLI is an experimental release, and is built on a best-efforts basis by some Auth0 developers in their available innovation time. It is open-source licensed and free to use, and is not covered by any Auth0 Terms of Service or Agreements. If you have issues with this CLI you can engage with the project's developer community through the repository GitHub Issues list, or contribute fixes and enhancements of your own via a Pull Request.

Build, test, troubleshoot and manage your integration with **[Auth0](http://auth0.com/)** directly from your **terminal**.

![demo](./demo.gif)

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Customization](#customization)
- [Anonymous Analytics](#anonymous-analytics)
- [Contributing](#contributing)
- [Author](#author)

---

## Features

### 🧪 Test the login flow at any time

You can easily test out the Universal Login box of your Auth0 application by running `auth0 test login`. This initiates a login flow in your browser. Once you complete the process, the Auth0 CLI will display your profile information and credentials.

### ⚡️ Get up and running quickly

You can also download a QuickStart sample application that’s already configured for your Auth0 application with `auth0 quickstarts download`. Just install the dependencies, and the sample application will be ready to run. Use it as an example integration to help set up Auth0 in your own application.

### 🔍 Find issues faster

If you encounter difficulties setting up your integration, use the Auth0 CLI to tail your tenant’s logs for a smoother troubleshooting experience. `auth0 logs tail` will let you inspect the authentication events as they happen. You can easily filter the events from a single Auth0 application with `--filter "client_id:<client-id>"` and use `--debug` to get the raw error details.

### 🔁 Simplify repetitive tasks

With the Auth0 CLI, you can:

- Manage your Auth0 applications, rules, and APIs right from the terminal.
- Create, update, and delete resources interactively.
- List all your resources or inspect them individually.

## Installation

Please install the `auth0-cli` in a way that matches your environment.

### macOS

#### [Homebrew](https://brew.sh/)

```bash
 brew tap auth0/auth0-cli && brew install auth0
```

### Windows

#### [Scoop](https://scoop.sh/)

```bash
scoop bucket add auth0 https://github.com/auth0/scoop-auth0-cli.git
scoop install auth0
```

### Linux

This can also be run on any platform that has [cURL](https://curl.se/) installed.

```bash
# Binary will be downloaded to "$(go env GOPATH)/bin/auth0".
curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/v1/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Or

# Binary will be downloaded to "./auth0".
curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/v1/install.sh | sh -s -- -b .
```

### Manual

1. Download the appropriate binary for your environment from the [latest release](https://github.com/auth0/auth0-cli/releases/latest/)
2. Extract the archive
   - **macOS**: `$ tar -xf auth0-cli_{version}_Darwin_{architecture}.tar.gz`
   - **Linux**: `$ tar -xf auth0-cli_{version}_Linux_{architecture}.tar.gz`
   - **Windows**: Extract `auth0-cli_{version}_Darwin_{architecture}.zip` using your preferred method of choice
3. Run `./auth0`
4. **_Optional for macOS / Linux_** - Setup CLI commands completion for your terminal:
   - (**bash**) `auth0 completion bash > /usr/local/etc/bash_completion.d/auth0`
   - (**zsh**) `auth0 completion zsh > "${fpath[1]}/_auth0"`
   - (**fish**) `auth0 completion fish | source`

> see more completion options by running the following command: `auth0 completion -h`

### Go users

```bash
go install github.com/auth0/auth0-cli/cmd/auth0@latest
```


## Usage

After installation, you should have the `auth0` command available:

```bash
auth0 [command]

# For any help, run --help after a specific command, e.g.:
auth0 [command] --help
```

- [auth0 actions](https://auth0.github.io/auth0-cli/auth0_actions.html) - Manage resources for actions
- [auth0 apis](https://auth0.github.io/auth0-cli/auth0_apis.html) - Manage resources for APIs
- [auth0 apps](https://auth0.github.io/auth0-cli/auth0_apps.html) - Manage resources for applications
- [auth0 attack-protection](https://auth0.github.io/auth0-cli/auth0_attack_protection.html) - Manage attack protection settings
- [auth0 branding](https://auth0.github.io/auth0-cli/auth0_branding.html) - Manage branding options
- [auth0 completion](https://auth0.github.io/auth0-cli/auth0_completion.html) - Setup autocomplete features for this CLI on your terminal
- [auth0 ips](https://auth0.github.io/auth0-cli/auth0_ips.html) - Manage blocked IP addresses
- [auth0 login](https://auth0.github.io/auth0-cli/auth0_login.html) - Authenticate the Auth0 CLI
- [auth0 logout](https://auth0.github.io/auth0-cli/auth0_logout.html) - Log out of a tenant's session
- [auth0 logs](https://auth0.github.io/auth0-cli/auth0_logs.html) - View tenant logs
- [auth0 orgs](https://auth0.github.io/auth0-cli/auth0_orgs.html) - Manage resources for organizations
- [auth0 quickstarts](https://auth0.github.io/auth0-cli/auth0_quickstarts.html) - Quickstart support for getting bootstrapped
- [auth0 roles](https://auth0.github.io/auth0-cli/auth0_roles.html) - Manage resources for roles
- [auth0 rules](https://auth0.github.io/auth0-cli/auth0_rules.html) - Manage resources for rules
- [auth0 tenants](https://auth0.github.io/auth0-cli/auth0_tenants.html) - Manage configured tenants
- [auth0 test](https://auth0.github.io/auth0-cli/auth0_test.html) - Try your Universal Login box or get a token
- [auth0 users](https://auth0.github.io/auth0-cli/auth0_users.html) - Manage resources for users

### Onboarding Journey

Following these instructions will give you a sense of what's possible with the
Auth0 CLI. To start, you will have to log in:

#### Login

To log in, run:

```bash
auth0 login
```

> **Warning**
> Authenticating as a user via `auth0 login` is not supported for **private cloud** tenants. Instead, those users should authenticate with client credentials via `auth0 tenants add`.

#### Creating your application

If you haven't created an application yet, you may do so by running the
following command:

```bash
auth0 apps create
```

A screen similar to the following will be presented after successful app creation:

```bash
$ auth0 apps create
 Name: My Awesome App
 Description: Test app
 Type: Regular Web Application
 Callback URLs: http://localhost:3000
 Allowed Logout URLs: http://localhost:3000

=== travel0.auth0.com application created

  CLIENT ID            wmVzrZkGhKgglMRMvpauORCulBkQ5qeI
  NAME                 My Awesome App
  DESCRIPTION          Test app
  TYPE                 regular web application
  CLIENT SECRET        kaS2NR5nk2PcGuITQ8JoKnpVnc5ky1TuKgsb6iTA08ec8XqizqkDupKhEIcsFiNM
  CALLBACKS            http://localhost:3000
  ALLOWED LOGOUT URLS  http://localhost:3000
  ALLOWED ORIGINS
  ALLOWED WEB ORIGINS
  TOKEN ENDPOINT AUTH
  GRANTS               implicit, authorization_code, refresh_token, client_credentials

 ▸    Quickstarts: https://auth0.com/docs/quickstart/webapp
 ▸    Hint: Test this app's login box with 'auth0 test login wmVzrZkGhKgglMRMvpauORCulBkQ5qeI'
 ▸    Hint: You might wanna try 'auth0 quickstarts download wmVzrZkGhKgglMRMvpauORCulBkQ5qeI'
```

As you might observe, the next thing to do would likely be to try logging in
using the client ID.

#### Testing the login flow

Whether or not you've created the application using the CLI or the management
dashboard, you'll be able to test logging in using a specific application.

If you have the client ID, you may pass it as an argument,
otherwise a prompt will be presented:

```bash
auth0 test login
```

#### Tailing your logs

Once you have a few logins in place, you might wanna tail your logs. This is
done by running the following command:

```bash
auth0 logs tail
```

After running that, one might see the following output:

```
Success Login   9 minutes ago  Username-Password-Authentic...    my awesome app
```

If there are errors encountered, such as the following example, you may run it
with the `--debug` flag as follows:

```bash
auth0 logs tail --debug
```

The full raw data will be displayed below every error:

```
Failed Login	hello	7 minutes ago	N/A	my awesome app

	id: "90020210306002808976000921438552554184272624146777636962"
	logid: "90020210306002808976000921438552554184272624146777636962"
	date: 2021-03-06T00:28:04.91Z
	type: f
	clientid: vXAtoaFdhlmtWjpIrjb9AUnrGEAOH2MM
	clientname: my awesome app
	ip: 1.2.3.4
	description: hello
	locationinfo: {}
	details:
	  body:
	    action: default
	    password: '*****'
	    state: QXV0aDAgaXMgaGlyaW5nISBhdXRoMC5jb20vY2FyZWVycyAK
	    username: j.doe@gmail.com
	  connection: Username-Password-Authentication
	  error:
	    message: hello
	    oauthError: access_denied
	    type: oauth-authorization
	  qs: {}
	  session_id: QXV0aDAgaXMgaGlyaW5nISBhdXRoMC5jb20vY2FyZWVycyAK
	userid: auth0|QXV0aDAgaXMgaGlyaW5nISBhdXRoMC5jb20vY2FyZWVycyAK
```

## Customization

To change the text editor used for editing templates, rules, and actions,
set the environment variable `EDITOR` to your preferred editor:

`export EDITOR="code -w"`

`export EDITOR="nano"`

## Anonymous Analytics

By default, the CLI tracks some anonymous usage events. This helps us understand how the CLI is being used, so we can continue to improve it. You can opt-out by setting the environment variable `AUTH0_CLI_ANALYTICS` to `false`.

### Data sent

Every event tracked sends the following data along with the event name:

- The CLI version.
- The OS name: as determined by the value of `GOOS`, e.g. `windows`.
- The processor architecture: as determined by the value of `GOARCH`, e.g. `amd64`.
- The install ID: an anonymous UUID that is stored in the CLI's config file.
- A timestamp.

## Contributing

Please check the [contributing guidelines](CONTRIBUTING.md).

## Author

[Auth0](https://auth0.com)

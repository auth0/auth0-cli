# Auth0 CLI (Experimental)

`auth0` is the command line to supercharge your development workflow. 

> Note: This CLI is an experimental release, and is built on a best-efforts basis by some Auth0 developers in their available innovation time. It is open-source licensed and free to use, and is not covered by any Auth0 Terms of Service or Agreements. If you have issues with this CLI you can engage with the project's developer community through the repository GitHub Issues list, or contribute fixes and enhancements of your own via a Pull Request.

Build, test, troubleshoot and manage your integration with **[Auth0](http://auth0.com/)** directly from your **terminal**.

![demo](./demo.gif)

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

### macOS

#### [Homebrew](https://brew.sh/)

```bash
 brew tap auth0/auth0-cli && brew install auth0
```

#### Manually

1. Download the _Darwin_ binary from the latest release: https://github.com/auth0/auth0-cli/releases/latest/
1. Extract
1. Run `./auth0`
1. Setup CLI commands completion for your terminal:
	-  (**bash**) `auth0 completion bash > /usr/local/etc/bash_completion.d/auth0`
	-  (**zsh**)  `auth0 completion zsh > "${fpath[1]}/_auth0"`
	- (**fish**)  `auth0 completion fish | source`

> see more completion options: `auth0 completion -h`

### Windows

#### [Scoop](https://scoop.sh/)

```bash
scoop bucket add auth0 https://github.com/auth0/scoop-auth0-cli.git
scoop install auth0
```

#### Manually

1. Download the _Windows_ binary from the latest release: https://github.com/auth0/auth0-cli/releases/latest/
1. Extract
1. Run `auth0.exe`

### Linux

#### Manually

1. Download the _Linux_ binary from the latest release: https://github.com/auth0/auth0-cli/releases/latest/
1. Extract `$  tar -xf auth0-cli_{dowloaded version here}_Linux_x86_64.tar.gz`
1. Run `./auth0` 
1. Setup CLI commands completion for your terminal:
	-  `sudo ./auth0 completion bash > /etc/bash_completion.d/auth0`
> see more completion options: `auth0 completion -h` 

## Usage

After installation, you should have the `auth0` command available:

```bash
auth0 [command]

# For any help, run --help after a specific command, e.g.:
auth0 [command] --help
```

* [auth0 apis](docs/auth0_apis.md)	 - Manage resources for APIs
* [auth0 apps](docs/auth0_apps.md)	 - Manage resources for applications
* [auth0 branding](docs/auth0_branding.md)	 - Manage branding options
* [auth0 completion](docs/auth0_completion.md)	 - Setup autocomplete features for this CLI on your terminal
* [auth0 ips](docs/auth0_ips.md)	 - Manage blocked IP addresses
* [auth0 login](docs/auth0_login.md)	 - Authenticate the Auth0 CLI
* [auth0 logout](docs/auth0_logout.md)	 - Log out of a tenant's session
* [auth0 logs](docs/auth0_logs.md)	 - View tenant logs
* [auth0 quickstarts](docs/auth0_quickstarts.md)	 - Quickstart support for getting bootstrapped
* [auth0 roles](docs/auth0_roles.md)	 - Manage resources for roles
* [auth0 rules](docs/auth0_rules.md)	 - Manage resources for rules
* [auth0 tenants](docs/auth0_tenants.md)	 - Manage configured tenants
* [auth0 test](docs/auth0_test.md)	 - Try your Universal Login box or get a token
* [auth0 users](docs/auth0_users.md)	 - Manage resources for users

### Onboarding Journey

Following these instructions will give you a sense of what's possible with the
Auth0 CLI. To start, you will have to login:

#### Login

```bash
auth0 login
```

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

The authenticator of the CLI defaults to the default Auth0 cloud `auth0.auth0.com`. This can be customized for personalized cloud offerings by setting the following env variables:

```
	AUTH0_AUDIENCE - The audience of the Auth0 Management API (System API) to use.
	AUTH0_CLIENT_ID - Client ID  of an application configured with the Device Code grant type.
	AUTH0_DEVICE_CODE_ENDPOINT - Device Authorization URL
	AUTH0_OAUTH_TOKEN_ENDPOINT - OAuth Token URL
```

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

# Auth0 CLI (Experimental)

`auth0` is the command line to supercharge your development workflow. 

> Note: This CLI is currently in an experimental state and is not supported by Auth0. It has not had a complete security review, and we do not recommend using it to interact with production tenants.

Build, test, and manage your integration with **[Auth0](http://auth0.com/)** directly from your **terminal**.

![demo](./demo.gif)


## Installation

### macOS

#### Homebrew

```
brew install auth0/auth0-cli/auth0
```

#### Manually

1. Download the binaries from: https://github.com/auth0/auth0-cli/releases/latest/
1. Extract
1. Move `auth0` to `/usr/local/bin/auth0`, e.g.: `mv ~/Desktop/auth0 /usr/local/bin`
1. Setup CLI commands completion for your terminal:
	-  (**bash**) `auth0 completion bash > /usr/local/etc/bash_completion.d/auth0`
	-  (**zsh**)  `auth0 completion zsh > "${fpath[1]}/_auth0"`
	- (**fish**)  `auth0 completion fish | source`

> see more completion options: `auth0 completion -h`

## Usage

After installation, you should have the `auth0` command available:

```bash
auth0 [command]

# For any help, run --help after a specific command, e.g.:
auth0 [command] --help
```

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
 Name: my awesome app
 Type: Regular Web Application
 Description: dev tester
Creating application... done

=== travel0 application created

  NAME           my awesome app
  TYPE           regular web application
  CLIENT ID      vXAtoaFdhlmtWjpIrjb9AUnrGEAOH2MM
  CLIENT SECRET  QXV0aDAgaXMgaGlyaW5nISBhdXRoMC5jb20vY2FyZWVycyAK

 ▸    Quickstarts: https://auth0.com/docs/quickstart/webapp
 ▸    Hint: You might wanna try `auth0 test login --client-id vXAtoaFdhlmtWjpIrjb9AUnrGEAOH2MM`
```

As you might observe, the next thing to do would likely be to try logging in
using the client ID.

#### Testing the login flow

Whether or not you've created the application using the CLI or the management
dashboard, you'll be able to test logging in using a specific application.

If you have the client ID, you may specify it via the `--client-id` flag,
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

## Contributing

Please check the [contributing guidelines](CONTRIBUTING.md).


## Author

[Auth0](https://auth0.com)

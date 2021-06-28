# Auth package

The CLI authentication follows this approach:

1. `$ auth0 login` uses **Auth0 Device Flow** to get an `acccess token` and a `refresh token` for the selected tenant.
1. The access token is stored at the configuration file.
1. The refresh token is stored at the OS keychain (supports macOS, Linux, and Windows thanks to https://github.com/zalando/go-keyring).
1. During regular commands initialization, the access token is used to instantiate an Auth0 API client. 
		- If the token is expired according to the value stored on the configuration file, a new one is requested using the refresh token. 
		- In case of any error, the interactive login flow is triggered.


### Customization

The authenticator the CLI uses defaults to the default Auth0 cloud offering of `auth0.auth0.com`. This can be customized for personalized cloud offerings by setting the following env variables: 
```
	AUTH0_AUDIENCE - The audience of the Auth0 Management API (System API) to use.
	AUTH0_CLIENT_ID - Client ID  of an application configured with the Device Code grant type.
	AUTH0_DEVICE_CODE_ENDPOINT - Device Authorization URL
	AUTH0_OAUTH_TOKEN_ENDPOINT - OAuth Token URL
```
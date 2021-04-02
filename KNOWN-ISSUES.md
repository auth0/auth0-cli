# Known Issues

1. [Storing the refresh token is not suported on WSL](#1-storing-the-refresh-token-is-not-suported-on-wsl)

## 1. Storing the refresh token is not suported on WSL

WSL users will get the following message upon successful login:
`Could not store the refresh token locally, please expect to login again once your access token expired.`

The Auth0 CLI uses the [go-keyring](https://github.com/zalando/go-keyring) library to securely store the refresh token across different operating systems. This is so every time the login credentials expire, the CLI can just silently renew them using the stored refresh token instead of having the user login again. However, that library [does not support WSL](https://github.com/zalando/go-keyring/issues/54), so the CLI will not store the refresh token on WSL. That means WSL users will have to log in again whenever the login credentials expire.

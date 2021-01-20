## v5.8.0

* `management.Ticket`: Add field `IncludeEmailInRedirect` ([#180](https://github.com/go-auth0/auth0/pull/180))
* `management.UserIdentity`: Add field `RefreshToken` ([#167](https://github.com/go-auth0/auth0/pull/167))

## v5.7.0

* `management.ClientRefreshToken`: Add `InfiniteTokenLifetime`, `InfiniteIdleTokenLifetime` and `IdleTokenLifetime` fields ([#184](https://github.com/go-auth0/auth0/pull/184)).
* `management.User`: Fix possible inconsistencies with `EmailVerified` field ([#181](https://github.com/go-auth0/auth0/pull/181)).

## v5.6.0

* `management.UserManager`: `Link()` links two user accounts together ([#178](https://github.com/go-auth0/auth0/pull/178)).

## v5.5.0

* `management.HookSecrets`: `HookSecrets` are not a pointer anymore ([#176](https://github.com/go-auth0/auth0/pull/176)).
* `management.HookManager`: `ReplaceSecrets` is introduced to enable PUT semantics for hook secrets ([#176](https://github.com/go-auth0/auth0/pull/176)).
* `management.TenantUniversalLoginColors`: Marshal/unmarshal branding colors correctly ([#177](https://github.com/go-auth0/auth0/pull/177)).

## v5.4.0

* `management.Branding`: add support for `BrandingUniversalLogin` ([#161](https://github.com/go-auth0/auth0/pull/161)).

## v5.3.0

* `management.ConnectionOptions`: unmarshal options only if not nil ([#165](https://github.com/go-auth0/auth0/pull/165)).
* `management.HookManager.List`: was using the wrong endpoint ([162](https://github.com/go-auth0/auth0/pull/162)).
* `management.ResourceServerManager.List`: was using the wrong endpoint.

## v5.2.2

* `management.UserIdentity`: Add `AccessToken` field ([#113](https://github.com/go-auth0/auth0/pull/113)).
* `management.ConnectionOptionsSAML`: Add missing `SetUserAttributes` field ([#159](https://github.com/go-auth0/auth0/pull/159)).

## v5.2.1

* `management.WithInsecure`: allow insecure HTTP scheme to enable testing / mocking.

## v5.2.0

* `management.LogStream`: new resource now available ([#144](https://github.com/go-auth0/auth0/pull/144))

## v5.1.0

* `management.Tenant`: changed `SessionTimeout` and `IdleSessionTimeout` to `float64`. Values smaller than 1 will be marshalled with a `_in_minutes` suffix ([#156](https://github.com/go-auth0/auth0/pull/156)).
* `management.Connection`: removed `RawOptions` and handle (un-)marshalling internally.

## v5.0.0

* `management.RequestOption`: renamed from `management.ListOption` and is now used with all requests to the Auth0 Management API ([#151](https://github.com/go-auth0/auth0/pull/151)).
* `management.ManagementOption`: renamed from the private `apiOption` and added several more options.
* `management.New`: now takes only one mandatory argument (domain). Authentication can is configured using `WithClientCredentials` or `WithStaticToken`.
* `management.Request`, `management.NewRequest` and `management.Do`: exposed a set of functions to allow more control over the request/response.

## v4.7.0

* `management.Job`: Fix typo in JSON tag ([#154](https://github.com/go-auth0/auth0/pull/154)).
* `management.ConnectionOptionsOAuth2`: Add `AuthorizationURL` and `TokenURL` ([#147](https://github.com/go-auth0/auth0/pull/147)).

## v4.6.0

* `management.ConnectionOptions`: Now supports `OAuth2` connection type ([#141](https://github.com/go-auth0/auth0/pull/141)).
* `management.ConnectionOptionsSAML`: Add missing options ([#138](https://github.com/go-auth0/auth0/pull/138/)).

## v4.5.0

* `management.User`: add `LastIP` and `LoginsCount` fields ([#137](https://github.com/go-auth0/auth0/pull/137)).

## v4.3.6

* `management.ConnectionOptionsOIDC`: add missing `Scopes()` and `SetScopes()` methods.

## v4.3.5

* `management.ConnectionOptions*`: `SetScopes()` was ignoring the `enable` argument.

## v4.2.0

* `management.UserManager`: `Roles()` returns `RoleList` ([#109](https://github.com/go-auth0/auth0/pull/109)).
* `management.UserManager`: `Permissions()` returns `PermissionList`.
* `management.RoleManager`: `Users()` returns `UserList`.
* `management.RoleManager`: `Permissions()` returns `PermissionList`.

## v4.1.1

* `management.Branding`: Support for both `BrandingColors.PageBackgroundGradient` as well as `BrandingColors.PageBackground`. ([#99](https://github.com/go-auth0/auth0/pull/99))

## v4.1.0

* `management.ConnectionOptionsEmail`, `management.ConnectionOptionsSMS`: add `authParams`.
* `management.UserIdentity`: correctly marshal/unmarshal integer `user_id`'s ([#101](https://github.com/go-auth0/auth0/issues/101), [#102](https://github.com/go-auth0/auth0/pull/102)). 

## v4.0.1

* `management.Tenant`: Add `use_scope_descriptions_for_consent` flag.

## v4.0.0

* **Breaking Change:** `Connection.Options` is now an `interface{}` accepting different types depending on the strategy.
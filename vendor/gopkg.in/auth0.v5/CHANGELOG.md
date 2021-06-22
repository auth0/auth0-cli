# v5.19.1

* Update README.md to reflect Actions and Organizations support.

# v5.19.0

* `management.Organizations`: Support for Auth0 [Organizations](https://auth0.com/docs/organizations) ([#216](https://github.com/go-auth0/auth0/pull/216))
* `management.CustomDomains`: Support for TLS Policy & Custom Domain IP Header ([#222](https://github.com/go-auth0/auth0/pull/222))

# v5.18.0

* `management.Actions`: Support for Auth0 ([Actions](https://auth0.com/docs/actions)) ([#219](https://github.com/go-auth0/auth0/pull/219))

## v5.17.0

* `management.AnomalyManager`: Support for `CheckIP` and `UnblockIP` ([#214](https://github.com/go-auth0/auth0/pull/214))

## v5.16.0

* `management.Connection`: Add support for Google Apps enterprise connection ([#212](https://github.com/go-auth0/auth0/pull/212))

## v5.15.0

* `management.Guardian`: Fixed minor bug when unmarshalling Policy string array ([#211](https://github.com/go-auth0/auth0/pull/211))

NOTES:

* Added support for .env config file support ([#210](https://github.com/go-auth0/auth0/pull/210))

## v5.14.1

* `management.Connection`: Add support for Auth0 SAML connection entityId option ([#209](https://github.com/go-auth0/auth0/pull/209))

## v5.14.0

* `management.SigningKey`: Add support for listing, reading, rotating and revoking an Auth0 Tenant's signing key ([#191](https://github.com/go-auth0/auth0/pull/191))
* `management.Grant`: Support pagination response when listing Grants ([#205](https://github.com/go-auth0/auth0/pull/205))
* `management.Guardian`: Support to create a MFA enrollment ticket ([#207](https://github.com/go-auth0/auth0/pull/207))
* `management.User`: Support to retrieve blocked IP addresses & unblock users using username, phone number or email ([#208](https://github.com/go-auth0/auth0/pull/208))
* `management.Connection`: Support deny list (non persistent attributes) of attributes Auth0 stores per connection object ([#203](https://github.com/go-auth0/auth0/pull/203))

## v5.13.0

* `management.ConnectionOptionsOIDC`: Add `SetUserAttributes` to OIDC Strategy

## v5.12.0

* `management.Connection`: Change Metadata property to be string map ([#160](https://github.com/go-auth0/auth0/pull/160))
* `management.Connection`: Add Auth0 managed `ProvisioningTicketUrl to root connection object ([#201](https://github.com/go-auth0/auth0/pull/201))
* `management.ConnectionOptions`: Adds missing AzureAD connection `TrustEmailVerified` option ([#198](https://github.com/go-auth0/auth0/pull/198))
* `management.ConnectionOptions`: Add missing user fields provided by Twitter ([#197](https://github.com/go-auth0/auth0/pull/197))
* `management.Guardian`: Add MFA Enrollment to Guardian ([#157](https://github.com/go-auth0/auth0/pull/157))
* `management.Guardian`: Extend MFA SMS Providers ([#200](https://github.com/go-auth0/auth0/pull/200))
* `management.Log`: Add description field to Logs ([#188](https://github.com/go-auth0/auth0/pull/188))
* `management.Prompt`: Adds `identifier_first` to New Universal Login Prompt ([#187](https://github.com/go-auth0/auth0/pull/187))
* `management.UserIdentity`: Add `AccessTokenSecret` to User Identity ([#196](https://github.com/go-auth0/auth0/pull/196))
* `management.UserIdentity`: Add MFA Enrollments for a User ([#157](https://github.com/go-auth0/auth0/pull/157))

## v5.11.0

* `management.ConnectionOptions`: Adds SetUserAttributes to ConnectionOptions to connections ([#194](https://github.com/go-auth0/auth0/pull/194))

## v5.10.0

* `management.ConnectionOptions`: Adds set_user_root_attributes to more connections ([#192](https://github.com/go-auth0/auth0/pull/192))

## v5.9.0 

* `management.ConnectionManager`: Return nil when ConnectionManager.ReadByName() name field is empty ([#186](https://github.com/go-auth0/auth0/pull/186))

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

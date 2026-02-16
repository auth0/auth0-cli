# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

# [v.1.27.2](https://github.com/auth0/auth0-cli/tree/v1.27.2) (February 14, 2026)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.27.1...v1.27.2)

### Fixed
- Fix `auth0 quickstarts setup` command's port assessment logic [#1440]


# [v.1.27.1](https://github.com/auth0/auth0-cli/tree/v1.27.1) (February 13, 2026)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.27.0...v1.27.1)

### Fixed
- Fix `auth0 quickstarts setup` command's interactive mode [#1437]


# [v.1.27.0](https://github.com/auth0/auth0-cli/tree/v1.27.0) (February 10, 2026)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.26.0...v1.27.0)

### Added
- Support for managing organization invitations via `auth0 orgs invitations` (list, show, create, delete) [#1424]
- New `auth0 quickstarts setup` command to scaffold Auth0 apps with `.env` generation (supports `vite`, `nextjs`) [#1428]
- Support for `auth0_prompt_screen_partial` in `auth0 tf generate` [#1426]
- Add missing custom text prompts in `auth0 tf generate -r auth0_prompt_custom_text` [#1426]
- Add `pre-login-organization-picker` screen to organizations prompt [#1426]
- Add required scopes for `organization_discovery_domains`, `self_service_profiles`, `user_attribute_profiles` [#1426]


# [v.1.26.0](https://github.com/auth0/auth0-cli/tree/v1.26.0) (January 14, 2026)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.25.1...v1.26.0)

### Added
- Handle forbidden error template fetching in `auth0 tf generate -r auth0_email_templates` [#1417]
- Handle GitHub rate-limits in `auth0 acul commands` by falling back to stable versions [#1414]

### Fixed
- Updated ULP branding assets to fix template playground language dropdown [#1416]


# [v.1.25.1](https://github.com/auth0/auth0-cli/tree/v1.25.1) (December 23, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.25.0...v1.25.1)

### Fixed
- Fix supported screens in advanced rendering flows [#1409]


# [v.1.25.0](https://github.com/auth0/auth0-cli/tree/v1.25.0) (December 16, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.24.0...v1.25.0)

### Added
- Add support for managing Token Exchange Profiles via `auth0 token-exchange` (EA-Only) [#1406]


# [v.1.24.0](https://github.com/auth0/auth0-cli/tree/v1.24.0) (December 09, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.23.0...v1.24.0)

### Added
- Support new ACUL config commands [#1312]
- Enhance login process by prompting for default tenant domain [#1388]
- Add interactive api selection for resource server client creation [#1391]
- Add resource server identifier flag alias for roles permissions commands [#1398]


# [v.1.23.0](https://github.com/auth0/auth0-cli/tree/v1.23.0) (November 14, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.22.0...v1.23.0)

### Added
- Add filter and paginated listing to `auth0 domains list` (EA-only) [#1365]
- Add support for generating `auth0_organization_discovery_domains` in `auth0 tf generate` [#1349]

### Fixed
- Fixed quickstart download failures caused by invalid zip responses [#1372]

# [v.1.22.0](https://github.com/auth0/auth0-cli/tree/v1.22.0) (October 21, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.21.0...v1.22.0)

### Added
- Add support for `auth0_branding_theme` in `auth0 tf generate` [#1366]
- Add support for token vault in `auth0 apps create` and `auth0 apps show <client-id>` [#1352]
- Add support for interactive support for comparing action versions in `auth0 actions diff <action-id>` [#1351]

### Fixed
- Fix missing email templates in terraform fetcher [#1362]
- Fix prevent and recover from JWT token corruption in keyring storage [#1358]
- Fix `auth apis create` subject type authorization validation [#1361]


# [v.1.21.0](https://github.com/auth0/auth0-cli/tree/v1.21.0) (September 30, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.20.1...v1.21.0)

### Added
- Add support for `auth0_self_service_profile` and `auth0_self_service_profile_custom_text` in `auth0 tf generate` [#1337]
- Add support for `auth0_user_attribute_profile` in `auth0 tf generate` [#1344]


# [v.1.20.1](https://github.com/auth0/auth0-cli/tree/v1.20.1) (September 25, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.20.0...v1.20.1)

### Fixed
- Fixed tenant re-authentication invocation when logging in with additional scopes. [#1343]


# [v.1.20.0](https://github.com/auth0/auth0-cli/tree/v1.20.0) (September 11, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.19.0...v1.20.0)

### Added
- Add support patch for network ACL update flow [#1265]

# [v.1.19.0](https://github.com/auth0/auth0-cli/tree/v1.19.0) (September 9, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.18.0...v1.19.0)

### Added
- Add support for managing `Async Approval` email template [#1317]
- Add support for Events: test triggers, redeliver, check failed deliveries and stats [#1288] 

### Fixed
- Handle 402 Payment error for specific resources during terraform generate [#1313]


# [v.1.18.0](https://github.com/auth0/auth0-cli/tree/v1.18.0) (September 2, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.17.1...v1.18.0)

### Added
- Add support for `subject-type-authorization` flag to apis commands. [#1315]

### Fixed
- Fix log ID retrieval and adjust sleep duration in logs processing. [#1314]


# [v.1.17.1](https://github.com/auth0/auth0-cli/tree/v1.17.1) (August 12, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.17.0...v1.17.1)

### Fixed
- Updated custom domain retrieval to use the `LIST` endpoint instead of `ListWithPagination`. [#1306]


# [v.1.17.0](https://github.com/auth0/auth0-cli/tree/v1.17.0) (August 07, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.16.0...v1.17.0)

### Added
- Updated ULP branding assets to support standard customization to latest [#1268]
- Improve error messaging for missing token scopes [#1247]
- Add json-compact Flag and improve JSON Output Handling [#1283]
- Add support for `pii-config` and `filters` flags to log-streams commands [#1280]


# [v.1.16.0](https://github.com/auth0/auth0-cli/tree/v1.16.0) (July 15, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.15.0...v1.16.0)

### Added
- Support for Multiple Custom Domains via `auth0 domains create` [#1240]


# [v.1.15.0](https://github.com/auth0/auth0-cli/tree/v1.15.0) (June 30, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.14.1...v1.15.0)

### Added
- Support for generating test tokens using a custom domain via `auth0 test token -d domain.auth0.com` [#1237]
- Added logic to handle analytics based on the type of login method [#1245]
- Support for private key JWT authentication [#1254]
- Support for new screens in Advanced Customization for Universal Login [#1258]
- Support for new schema fields in Advanced Customization for Universal Login [#1260]


# [v.1.14.1](https://github.com/auth0/auth0-cli/tree/v1.14.1) (May 27, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.14.0...v1.14.1)

### Fixed:
- Remove unreleased screens for ACUL [#1231] 


# [v.1.14.0](https://github.com/auth0/auth0-cli/tree/v1.14.0) (May 22, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.13.0...v1.14.0)

### Added
- New command to manage tenant flags via `auth0 tenant-settings show/update` [#1203]
- Support for new screens in Advanced Customization for Universal Login [#1225]
- Subcommand `search-by-email` on `auth0 users` along with --picker flag [#1209]

### Fixed
- Respect --screen flag for `auth0 ul customize` [#1228]
- Replace package `mholt/archiver` with custom implementation of unzip [#1218]


# [v.1.13.0](https://github.com/auth0/auth0-cli/tree/v1.13.0) (May 7, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.12.0...v1.13.0)

### Added
- New flag for improved visualization of logs using `auth0 logs ls -p` [#1195]
- Support to manage session-transfer for applications using `auth0 apps session-transfer` [#1180]
- Support to set `refresh-token` for a client and configure Multi Resource Refresh Token [#1192]

### Fixed
- Handle 403 forbidden during `auth0 tf generate` for non feature-flag enabled tenants [#1197]

# [v.1.12.0](https://github.com/auth0/auth0-cli/tree/v1.12.0) (Apr 28, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.11.0...v1.12.0)

### Added
- Support to manage tenant ACL using `auth0 network-acl` (EA Release) [#1166]
- Add support for new screens in Advanced Customization for Universal Login [#1185]
- Support authentication blocking for an user via `auth0 users update <user-id> --blocked`[#1181]
- Support additional scopes for connections [#1184]

### Changed
- Updated ULP branding assets to support standard customization of Universal login for all the available prompts[#1188]

### Fixed
- Fix validation to authorize audience only for M2M apps in test commands [#1183]


# [v.1.11.0](https://github.com/auth0/auth0-cli/tree/v1.11.0) (Apr 02, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.10.1...v1.11.0)

### Added

- Support org flag in test login and test token command [#1173]

### Fixed

- Update assets related to universal login[#1172]


# [v.1.10.1](https://github.com/auth0/auth0-cli/tree/v1.10.1) (Mar 28, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.10.0...v1.10.1)

### Added

- Add support for new screens in Advanced Customization for Universal Login [#1167]

### Fixed

- Handle nil check on customText cache in assets and update CDN textLocal URL [#1170]


# [v.1.10.0](https://github.com/auth0/auth0-cli/tree/v1.10.0) (Mar 11, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.9.3...v1.10.0)

### Added

- Add support to manage phone provider using `auth0 phone provider` [#1137]


# [v.1.9.3](https://github.com/auth0/auth0-cli/tree/v1.9.3) (Mar 07, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.9.2...v1.9.3)

### Fixed

- Handle nil check on ReadRendering management API Response [#1150]


# [v.1.9.2](https://github.com/auth0/auth0-cli/tree/v1.9.2) (Mar 05, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.9.1...v1.9.2)

### Added

- Optimize universal-login commands [#1142]

### Removed

- Remove unsupported query params from the domains list implementation [#1144]


# [v.1.9.1](https://github.com/auth0/auth0-cli/tree/v1.9.1) (Feb 21, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.9.0...v1.9.1)

### Added

- Add support for new screens in Advanced Customization for Universal Login [#1140]


# [v.1.9.0](https://github.com/auth0/auth0-cli/tree/v1.9.0) (Feb 6, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.8.0...v1.9.0)

### Added

- Add support for new screens in Advanced Customization for Universal Login [#1132]
- Add support to set custom url parameters using `--params` in `auth0 test` [#1130]
- Add support to set runtime using `--runtime` in `auth0 actions` [#1131]
- Add support to manage Event Streams using `auth0 events` [#1134]

### Changed

- Updated `auth0 ul customize` branding assets to load custom text based on selected screens [#1124]


# [v.1.8.0](https://github.com/auth0/auth0-cli/tree/v1.8.0) (Jan 21, 2025)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.7.2...v1.8.0)

### Added

- Support `reset_email_by_code` email template [#1119]
- Add support for configuring `email provider` [#1120]
- Add `requiredScopes` related to emailProvider [#1129]


# [v.1.7.2](https://github.com/auth0/auth0-cli/tree/v1.7.2) (Dec 19, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.7.1...v1.7.2)

### Fixed

- fix(terraform): Handle 403 forbidden error [#1115]


# [v.1.7.1](https://github.com/auth0/auth0-cli/tree/v1.7.1) (Dec 19, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.7.0...v1.7.1)

### Added

- Support flags for `auth0 ul customize` command to choose the renderingMode, prompt & screenNames along with configSettings file[#1111]

### Fixed

- Fix `auth0 tf generate` command and handle error when custom domain is not enabled [#1103]
- Fix CDN textLocal URL & include unit tests for fetchData of the resource `auth0_prompt_screen_renderer`[#1109]


# [v.1.7.0](https://github.com/auth0/auth0-cli/tree/v1.7.0) (Dec 9, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.6.1...v1.7.0)

### Added

- Support for importing `auth0_prompt_screen_renderer` terraform resource [#1106]

### Fixed

- For `ul login` added check to filter and identify only support partials. [#1107]

# [v1.6.1](https://github.com/auth0/auth0-cli/tree/v1.6.1) (Oct 31, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.6.0...v1.6.1)

### Added

- Added new flag (`tf-version`) to pass terraform version during `auth0 tf generate` command [#1098]

### Removed

- Removed iga-* triggers from triggerActionsResourceFetcher [#1099]

# [v1.6.0](https://github.com/auth0/auth0-cli/tree/v1.6.0) (Oct 17, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.5.1...v1.6.0)

### Added

- Support for importing `Flows`, `Forms` and `FlowVaultConnections` Terraform Resources [#1084]

### Fixed

- Resolved an issue to support `passwordless connection` while creating and updating a user  [#1091]

# [v1.5.1](https://github.com/auth0/auth0-cli/tree/v1.5.1) (Oct 4, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.5.0...v1.5.1)

### Fixed

- Resolved an issue with `auth0_resource_server_scopes` in the `auth0_resource_server` Terraform import when generating Terraform configuration [#1079]
- Improved error handling in `auth0 ul customize` to gracefully ignore specific prompt errors [#1081]
- Fixed an issue with the display of custom domains data and deletion across all commands that involve multiple pick options [#1083]

# [v1.5.0](https://github.com/auth0/auth0-cli/tree/v1.5.0) (Aug 13, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.4.0...v1.5.0)

### Added

- Support for `auth0_client_credentials` within `auth0_client` when generating Terraform configuration [#1032]
- Support for `auth0_email_template` when generating Terraform configuration [#988]
- Ability to use **Custom Partial Prompts** in the `auth0 universal-login customize` command [#1031]
- Ability to manage the login domain through the `--domain` flag in the `auth0 login` command with an updated login flow [#1038]

### Fixed

- Issue with listing tenants in JSON format. [#1002](https://github.com/auth0/auth0-cli/issues/1002)

## [v1.4.0](https://github.com/auth0/auth0-cli/tree/v1.4.0) (Feb 1, 2024)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.3.0...v1.4.0)

### Added

- Warning when the user is about to delete the client used to authenticate the CLI [#934]
- Ability to select multiple IDs when deleting a resource [#935]
- Support for multiple identifiers for blocks list and unblock commands [#931]
- Ability to manage client metadata through `--metadata` flag on apps create and update commands [#938]
- Allow piped-in templates when invoking the universal-login templates update command [#950]
- CSV format for list commands [#955]

### Fixed

- Listing organizations members in json format [#953]
- Prevent saving branding data in universal-login customize command if empty [#968]

### Changed

- Set default resources per page to 100 when invoking list commands [#940]
- Standardize error messages [#943]
- Progress indicator changed from a spinner icon to a progress bar across commands that work with bulk resources [#949]

## [v1.3.0](https://github.com/auth0/auth0-cli/tree/v1.3.0) (Dec 1, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.2.0...v1.3.0)

### Added

- Audience dropdown UI when running `auth0 test token` [#906]
- Scopes dropdown UI when running `auth0 test token` [#910]
- Active tenant indicator when running `auth0 tenants list` [#907]
- Prompt dropdown UI when running  `auth0 ul prompts update` and `auth0 ul prompts list` [#913]
- Signing algorithm management when running `auth0 apis create` and `auth0 apis update` [#926]
- Log stream dropdown UI when running `auth0 logs streams show` [#920]
- Validate connection is enabled before creating and importing users [#921]

### Fixed

- Display description field when updating roles in interactive mode [#915]
- Only store access token in config file if keyring unavailable [#919]
- Only display undeployed actions when running `auth0 actions deploy` [#916]
- Don't require any scopes when authenticating with client credentials [#917]
- Show help text when no arguments provided when running  `auth0 api` [#914]

### Changed

- Removal of "Getting members of organization" loader messaging when running `auth0 orgs members list` [#918]

## [v1.2.0](https://github.com/auth0/auth0-cli/tree/v1.2.0) (Nov 2, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.1.2...v1.2.0)

### Added

- `universal-login customize` command for customizing the branding for the new Universal Login Experience [#882]

## [v1.1.2](https://github.com/auth0/auth0-cli/tree/v1.1.2) (Sept 29, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.1.1...v1.1.2)

### Fixed

- Disallowing of mismatched Auth0 domain in Terraform provider when using `auth0 tf generate` [#858]
- Check if an email provider is configured before exporting with `auth0 tf generate` [#857]
- Check if a resource server has associated scopes before exporting `auth0_resource_server_scopes` with `auth0 tf generate` [#856]

## [v1.1.1](https://github.com/auth0/auth0-cli/tree/v1.1.1) (Sept 22, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.1.0...v1.1.1)

### Fixed

- Passing of multiple dependencies for `auth0 action` commands [#850]
- JSON unmarshalling error when testing login of Apple social connections [#851]
- Free-tiered tenants erroring when exporting custom domains with `auth0 tf generate` [#854]

### Changed

- Terraform provider version using latest version, 1.0.0 at minimum, for `auth0 tf generate` [#853]

## [v1.1.0](https://github.com/auth0/auth0-cli/tree/v1.1.0) (Sept 15, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.0.1...v1.1.0)

### Added

- `terraform generate` command for auto-generating Terraform configuration from your Auth0 tenant. Refer to the [Generate Terraform Config guide](https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/generate_terraform_config) for instructions on how to use. [#792]
- Retry for select HTTP error codes [#839]

### Fixed

- Passing of multiple secrets for `auth0 action` commands [#844]
- Show non-ready custom domains with `auth0 domains list` command [#781]

## [v1.0.1](https://github.com/auth0/auth0-cli/tree/v1.0.1) (Apr 20, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.0.0...v1.0.1)

### Fixed

- "Not logged in. Try 'auth0 login'." warnings when installing from package manager [#741]
- Unmarshaling log scopes error with `auth0 logs` commands [#745]
- Returning empty array when no results with `--json` flag [#747]
- Error occurring no tenant logs when running `auth0 logs` commands [#744]
- Always showing the hint to log in even if we are logged in [#743]

## [v1.0.0](https://github.com/auth0/auth0-cli/tree/v1.0.0) (Apr 14, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v0.13.1...v1.0.0)

:warning: Refer to the [v1 migration guide](MIGRATION_GUIDE.md) for instructions on how to navigate breaking changes.

### Added

- Ability to view user's assigned roles via `auth0 users roles show` [#604]
- Assign role(s) to user via `auth0 users roles assign` [#605]
- Remove user role(s) via `auth0 users roles remove` [#606]
- `perms` alias for `auth0 roles permissions` command [#534]
- Authenticating via client credentials with `auth0 login` [#546]
- Graceful access token regeneration [#547]
- Storing client secret in operating system keyring [#578]
- Supporting additional scopes through `--scopes` flag when authenticating as user [#538]
- Argument to specify log stream type for `auth0 logs streams create` and `auth0 logs streams update` [#599]
- Better guidance on authenticating in the `auth0 login` help text [#565]
- Confirmation prompts before applying editor updates [#603]
- Bespoke messaging when no logs match the provided filter criteria [#733]
- `--users` flag to `auth0 users import` command for providing user JSON payload [#735]
- Warning if updating universal login templates with classic mode enabled [#667]
- Automatic retries with `api` command [#681]
- Clearer device code comparison instructions [#664]
- Storing access token in OS keyring if possible [#645]
- DX improvements to `test login` and `test token` commands [#629]

### Fixed

- Return empty JSON array when no results for all list commands and the `--json` flag [#736]
- Unrequiring `--audience` flag in `auth0 test login` [#694]
- Removing duplicate header in `auth0 apis list` output [#711]
- Prevent panic in `auth0 ul templates update` if no branding settings exist [#731]
- Missing table header when using `auth0 logs tail` [#732]
- Empty dashboard urls during `open` commands when authenticated using client credentials [#652]
- `auth0 logs tail` terminating early if no logs found [#672]
- `auth0 apps list` rendering correct number of results in output header [#674]
- `auth0 test token` failing silently with invalid audience input [#671]
- Possible panic when running `auth0 ul update` with empty branding settings (ex: newly-created tenant) [#692]
- Inability to update user password with `auth0 users update --password` [#686]
- Apps shown in multi select when no app-id is passed [#648]
- "something went wrong" error during `auth0 branding texts update` [#584]
- Help text descriptions for most instances of `--number` flag [#610]
- Allow updating a non-existent email template with `auth0 email templates update` [#611]
- `--no-input` flag works for `auth0 test token` and `auth0 test login` commands [#613]
- `--no-color` flag works for all commands [#594]
- All available triggers present when running `auth0 actions create` [#597]
- Extraneous payload property when running `auth0 orgs update` [#583]
- Users search command enables pagination through `--number` flag [#588]
- Tenant commands now respect `--tenant` flag [#612]
- Log tail output now displays absolute time instead of relative [#590]
- Adding missing headers for `auth0 logs list` [#589]
- Output new action data when running `auth0 actions update` [#596]
- Log streams "no roles" errors message [#598]
- Removed erroneous `auth0 apis show --json` truncation message [#607]
- Skip interactive elements when `--json` and `--force` flags are passed [#616]

### Changed

- `--format json` flag/value pair consolidated to `--json` [#533]
- Flattened the `auth0 branding` commands into the root-level [#540], [#541]
- Moved `auth0 ips` command to `auth0 ap sit ips` [#618]
- Moved `auth0 users unblock` to `auth0 users blocks unblock` [#617]
- `--reveal` flag for reveal client secret renamed to `--reveal-secrets` [#591]
- More actionable output when executing `auth0 users import` [#735]
- Editorializing code "hints" throughout project [#570]
- Renamed `--connection` flag to `--connection-name` for `auth0 users` commands for consistency [#738]

### Removed

- `--force` and `--json` flags relegated from global context, now applied only where appropriate [#536], [#595]
- Undocumented `auth0 config init` command [#532]
- `auth0 tenants add` command in favor of `auth0 login` [#546]
- Updating of action triggers which inevitably results in error [#597]
- `-u` shortform alias for `--upsert` flag in `auth0 users import` [#735]

## [v1.0.0-beta.3](https://github.com/auth0/auth0-cli/tree/v1.0.0-beta.3) (Mar 30, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.0.0-beta.2...v1.0.0-beta.3)

### Added

- Re-adding storybook preview when updating universal login templates [#666]
- Warning if updating universal login templates with classic mode enabled [#667]
- Automatic retries with `api` command [#681]
- Clearer device code comparison instructions [#664]

### Fixed

- Empty dashboard urls during `open` commands when authenticated using client credentials [#652]
- `auth0 logs tail` terminating early if no logs found [#672]
- `auth0 apps list` rendering correct number of results in output header [#674]
- `auth0 test token` failing silently with invalid audience input [#671]
- Possible panic when running `auth0 ul update` with empty branding settings (ex: newly-created tenant) [#692]
- Inability to update user password with `auth0 users update --password` [#686]

## [v1.0.0-beta.2](https://github.com/auth0/auth0-cli/tree/v1.0.0-beta.2) (Feb 14, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.0.0-beta.1...v1.0.0-beta.2)

### Added

- Storing access token in OS keyring if possible [#645]
- DX improvements to `test login` and `test token` commands [#629]

### Fixed

- Apps shown in multi select when no app-id is passed [#648]

## [v1.0.0-beta.1](https://github.com/auth0/auth0-cli/tree/v1.0.0-beta.1) (Jan 20, 2023)

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v0.13.1...v1.0.0-beta.1)

### Added

- Ability to view user's assigned roles via `auth0 users roles show` [#604]
- Assign role(s) to user via `auth0 users roles assign` [#605]
- Remove user role(s) via `auth0 users roles remove` [#606]
- `perms` alias for `auth0 roles permissions` command [#534]
- Authenticating via client credentials with `auth0 login` [#546]
- Graceful access token regeneration [#547]
- Storing client secret in operating system keyring [#578]
- Supporting additional scopes through `--scopes` flag when authenticating as user [#538]
- Argument to specify log stream type for `auth0 logs streams create` and `auth0 logs streams update` [#599]
- Better guidance on authenticating in the `auth0 login` help text [#565]
- Confirmation prompts before applying editor updates [#603]

### Changed

- `--format json` flag/value pair consolidated to `--json` [#533]
- Flattened the `auth0 branding` commands into the root-level [#540], [#541]
- Moved `auth0 ips` command to `auth0 ap sit ips` [#618]
- Moved `auth0 users unblock` to `auth0 users blocks unblock` [#617]
- `--reveal` flag for reveal client secret renamed to `--reveal-secrets` [#591]
- Editorializing code "hints" throughout project [#570]

### Fixed

- "something went wrong" error during `auth0 branding texts update` [#584]
- Help text descriptions for most instances of `--number` flag [#610]
- Allow updating a non-existent email template with `auth0 email templates update` [#611]
- `--no-input` flag works for `auth0 test token` and `auth0 test login` commands [#613]
- `--no-color` flag works for all commands [#594]
- All available triggers present when running `auth0 actions create` [#597]
- Extraneous payload property when running `auth0 orgs update` [#583]
- Users search command enables pagination through `--number` flag [#588]
- Tenant commands now respect `--tenant` flag [#612]
- Log tail output now displays absolute time instead of relative [#590]
- Adding missing headers for `auth0 logs list` [#589]
- Output new action data when running `auth0 actions update` [#596]
- Log streams "no roles" errors message [#598]
- Removed erroneous `auth0 apis show --json` truncation message [#607]
- Skip interactive elements when `--json` and `--force` flags are passed [#616]

### Removed

- Storybook preview when updating universal login templates [#592]
- `--force` and `--json` flags relegated from global context, now applied only where appropriate [#536], [#595]
- Undocumented `auth0 config init` command [#532]
- `auth0 tenants add` command in favor of `auth0 login` [#546]
- Updating of action triggers which inevitably results in error [#597]


[unreleased]: https://github.com/auth0/auth0-cli/compare/v1.27.2...HEAD
[#1440]: https://github.com/auth0/auth0-cli/pull/1440
[#1437]: https://github.com/auth0/auth0-cli/pull/1437
[#1428]: https://github.com/auth0/auth0-cli/pull/1428
[#1426]: https://github.com/auth0/auth0-cli/pull/1426
[#1424]: https://github.com/auth0/auth0-cli/pull/1424
[#1417]: https://github.com/auth0/auth0-cli/pull/1417
[#1416]: https://github.com/auth0/auth0-cli/pull/1416
[#1414]: https://github.com/auth0/auth0-cli/pull/1414
[#1409]: https://github.com/auth0/auth0-cli/pull/1409
[#1406]: https://github.com/auth0/auth0-cli/pull/1406
[#1398]: https://github.com/auth0/auth0-cli/pull/1398
[#1391]: https://github.com/auth0/auth0-cli/pull/1391
[#1388]: https://github.com/auth0/auth0-cli/pull/1388
[#1312]: https://github.com/auth0/auth0-cli/pull/1312
[#1372]: https://github.com/auth0/auth0-cli/pull/1372
[#1365]: https://github.com/auth0/auth0-cli/pull/1365
[#1349]: https://github.com/auth0/auth0-cli/pull/1349
[#1344]: https://github.com/auth0/auth0-cli/pull/1344
[#1337]: https://github.com/auth0/auth0-cli/pull/1337
[#1343]: https://github.com/auth0/auth0-cli/issues/1343
[#1265]: https://github.com/auth0/auth0-cli/issues/1265
[#1317]: https://github.com/auth0/auth0-cli/issues/1317
[#1313]: https://github.com/auth0/auth0-cli/issues/1313
[#1306]: https://github.com/auth0/auth0-cli/issues/1306
[#1288]: https://github.com/auth0/auth0-cli/issues/1288
[#1283]: https://github.com/auth0/auth0-cli/issues/1283
[#1280]: https://github.com/auth0/auth0-cli/issues/1280
[#1268]: https://github.com/auth0/auth0-cli/issues/1268
[#1260]: https://github.com/auth0/auth0-cli/issues/1260
[#1258]: https://github.com/auth0/auth0-cli/issues/1258
[#1254]: https://github.com/auth0/auth0-cli/issues/1254
[1247]: https://github.com/auth0/auth0-cli/issues/1247
[#1245]: https://github.com/auth0/auth0-cli/issues/1245
[#1240]: https://github.com/auth0/auth0-cli/issues/1240
[#1237]: https://github.com/auth0/auth0-cli/issues/1237
[#1231]: https://github.com/auth0/auth0-cli/issues/1231
[#1228]: https://github.com/auth0/auth0-cli/issues/1228   
[#1225]: https://github.com/auth0/auth0-cli/issues/1225
[#1218]: https://github.com/auth0/auth0-cli/issues/1218
[#1209]: https://github.com/auth0/auth0-cli/issues/1209
[#1203]: https://github.com/auth0/auth0-cli/issues/1203
[#1197]: https://github.com/auth0/auth0-cli/issues/1197
[#1195]: https://github.com/auth0/auth0-cli/issues/1195
[#1192]: https://github.com/auth0/auth0-cli/issues/1192
[#1188]: https://github.com/auth0/auth0-cli/issues/1188
[#1185]: https://github.com/auth0/auth0-cli/issues/1185
[#1184]: https://github.com/auth0/auth0-cli/issues/1184
[#1183]: https://github.com/auth0/auth0-cli/issues/1183
[#1182]: https://github.com/auth0/auth0-cli/issues/1182
[#1181]: https://github.com/auth0/auth0-cli/issues/1181
[#1180]:https://github.com/auth0/auth0-cli/issues/1180
[#1166]: https://github.com/auth0/auth0-cli/issues/1166
[#1173]: https://github.com/auth0/auth0-cli/issues/1173
[#1172]: https://github.com/auth0/auth0-cli/issues/1172
[#1170]: https://github.com/auth0/auth0-cli/issues/1170
[#1167]: https://github.com/auth0/auth0-cli/issues/1167
[#1137]: https://github.com/auth0/auth0-cli/issues/1137
[#1150]: https://github.com/auth0/auth0-cli/issues/1150
[#1144]: https://github.com/auth0/auth0-cli/issues/1144
[#1142]: https://github.com/auth0/auth0-cli/issues/1142
[#1140]: https://github.com/auth0/auth0-cli/issues/1140
[#1134]: https://github.com/auth0/auth0-cli/issues/1134
[#1132]: https://github.com/auth0/auth0-cli/issues/1132
[#1130]: https://github.com/auth0/auth0-cli/issues/1130
[#1131]: https://github.com/auth0/auth0-cli/issues/1131
[#1129]: https://github.com/auth0/auth0-cli/issues/1129
[#1124]: https://github.com/auth0/auth0-cli/issues/1124
[#1120]: https://github.com/auth0/auth0-cli/issues/1120
[#1119]: https://github.com/auth0/auth0-cli/issues/1119
[#1115]: https://github.com/auth0/auth0-cli/issues/1115
[#1111]: https://github.com/auth0/auth0-cli/issues/1111
[#1109]: https://github.com/auth0/auth0-cli/issues/1109
[#1103]: https://github.com/auth0/auth0-cli/issues/1103
[#1107]: https://github.com/auth0/auth0-cli/issues/1107
[#1106]: https://github.com/auth0/auth0-cli/issues/1106
[#1099]: https://github.com/auth0/auth0-cli/issues/1099
[#1098]: https://github.com/auth0/auth0-cli/issues/1098
[#1091]: https://github.com/auth0/auth0-cli/issues/1091
[#1084]: https://github.com/auth0/auth0-cli/issues/1084
[#1083]: https://github.com/auth0/auth0-cli/issues/1083
[#1081]: https://github.com/auth0/auth0-cli/issues/1081
[#1079]: https://github.com/auth0/auth0-cli/issues/1079
[#1038]: https://github.com/auth0/auth0-cli/issues/1038
[#1032]: https://github.com/auth0/auth0-cli/issues/1032
[#1031]: https://github.com/auth0/auth0-cli/issues/1031
[#1002]: https://github.com/auth0/auth0-cli/issues/1002
[#988]: https://github.com/auth0/auth0-cli/issues/988
[#968]: https://github.com/auth0/auth0-cli/issues/968
[#955]: https://github.com/auth0/auth0-cli/issues/955
[#949]: https://github.com/auth0/auth0-cli/issues/949
[#953]: https://github.com/auth0/auth0-cli/issues/953
[#950]: https://github.com/auth0/auth0-cli/issues/950
[#943]: https://github.com/auth0/auth0-cli/issues/943
[#940]: https://github.com/auth0/auth0-cli/issues/940
[#938]: https://github.com/auth0/auth0-cli/issues/938
[#931]: https://github.com/auth0/auth0-cli/issues/931
[#935]: https://github.com/auth0/auth0-cli/issues/935
[#934]: https://github.com/auth0/auth0-cli/issues/934
[#906]: https://github.com/auth0/auth0-cli/issues/906
[#910]: https://github.com/auth0/auth0-cli/issues/910
[#907]: https://github.com/auth0/auth0-cli/issues/907
[#913]: https://github.com/auth0/auth0-cli/issues/913
[#926]: https://github.com/auth0/auth0-cli/issues/926
[#920]: https://github.com/auth0/auth0-cli/issues/920
[#921]: https://github.com/auth0/auth0-cli/issues/921
[#915]: https://github.com/auth0/auth0-cli/issues/915
[#919]: https://github.com/auth0/auth0-cli/issues/919
[#916]: https://github.com/auth0/auth0-cli/issues/916
[#917]: https://github.com/auth0/auth0-cli/issues/917
[#914]: https://github.com/auth0/auth0-cli/issues/914
[#918]: https://github.com/auth0/auth0-cli/issues/918
[#882]: https://github.com/auth0/auth0-cli/issues/882
[#858]: https://github.com/auth0/auth0-cli/issues/858
[#857]: https://github.com/auth0/auth0-cli/issues/857
[#856]: https://github.com/auth0/auth0-cli/issues/856
[#850]: https://github.com/auth0/auth0-cli/issues/850
[#851]: https://github.com/auth0/auth0-cli/issues/851
[#854]: https://github.com/auth0/auth0-cli/issues/854
[#853]: https://github.com/auth0/auth0-cli/issues/853
[#792]: https://github.com/auth0/auth0-cli/issues/792
[#839]: https://github.com/auth0/auth0-cli/issues/839
[#844]: https://github.com/auth0/auth0-cli/issues/844
[#781]: https://github.com/auth0/auth0-cli/issues/781
[#743]: https://github.com/auth0/auth0-cli/issues/743
[#747]: https://github.com/auth0/auth0-cli/issues/747
[#745]: https://github.com/auth0/auth0-cli/issues/745
[#744]: https://github.com/auth0/auth0-cli/issues/744
[#741]: https://github.com/auth0/auth0-cli/issues/741
[#733]: https://github.com/auth0/auth0-cli/issues/733
[#738]: https://github.com/auth0/auth0-cli/issues/738
[#735]: https://github.com/auth0/auth0-cli/issues/735
[#736]: https://github.com/auth0/auth0-cli/issues/736
[#694]: https://github.com/auth0/auth0-cli/issues/694
[#711]: https://github.com/auth0/auth0-cli/issues/711
[#731]: https://github.com/auth0/auth0-cli/issues/731
[#732]: https://github.com/auth0/auth0-cli/issues/732
[#686]: https://github.com/auth0/auth0-cli/issues/686
[#692]: https://github.com/auth0/auth0-cli/issues/692
[#671]: https://github.com/auth0/auth0-cli/issues/671
[#667]: https://github.com/auth0/auth0-cli/issues/667
[#666]: https://github.com/auth0/auth0-cli/issues/666
[#674]: https://github.com/auth0/auth0-cli/issues/674
[#681]: https://github.com/auth0/auth0-cli/issues/681
[#664]: https://github.com/auth0/auth0-cli/issues/664
[#672]: https://github.com/auth0/auth0-cli/issues/672
[#652]: https://github.com/auth0/auth0-cli/issues/652
[#648]: https://github.com/auth0/auth0-cli/issues/648
[#645]: https://github.com/auth0/auth0-cli/issues/645
[#629]: https://github.com/auth0/auth0-cli/issues/629
[#592]: https://github.com/auth0/auth0-cli/issues/592
[#604]: https://github.com/auth0/auth0-cli/issues/604
[#605]: https://github.com/auth0/auth0-cli/issues/605
[#606]: https://github.com/auth0/auth0-cli/issues/606
[#534]: https://github.com/auth0/auth0-cli/issues/534
[#546]: https://github.com/auth0/auth0-cli/issues/546
[#547]: https://github.com/auth0/auth0-cli/issues/547
[#578]: https://github.com/auth0/auth0-cli/issues/578
[#538]: https://github.com/auth0/auth0-cli/issues/538
[#599]: https://github.com/auth0/auth0-cli/issues/599
[#565]: https://github.com/auth0/auth0-cli/issues/565
[#603]: https://github.com/auth0/auth0-cli/issues/603
[#533]: https://github.com/auth0/auth0-cli/issues/533
[#540]: https://github.com/auth0/auth0-cli/issues/540
[#541]: https://github.com/auth0/auth0-cli/issues/541
[#591]: https://github.com/auth0/auth0-cli/issues/591
[#570]: https://github.com/auth0/auth0-cli/issues/570
[#584]: https://github.com/auth0/auth0-cli/issues/584
[#610]: https://github.com/auth0/auth0-cli/issues/610
[#611]: https://github.com/auth0/auth0-cli/issues/611
[#613]: https://github.com/auth0/auth0-cli/issues/613
[#594]: https://github.com/auth0/auth0-cli/issues/594
[#597]: https://github.com/auth0/auth0-cli/issues/597
[#583]: https://github.com/auth0/auth0-cli/issues/583
[#588]: https://github.com/auth0/auth0-cli/issues/588
[#590]: https://github.com/auth0/auth0-cli/issues/590
[#589]: https://github.com/auth0/auth0-cli/issues/589
[#596]: https://github.com/auth0/auth0-cli/issues/596
[#598]: https://github.com/auth0/auth0-cli/issues/598
[#607]: https://github.com/auth0/auth0-cli/issues/607
[#616]: https://github.com/auth0/auth0-cli/issues/616
[#536]: https://github.com/auth0/auth0-cli/issues/536
[#532]: https://github.com/auth0/auth0-cli/issues/532
[#546]: https://github.com/auth0/auth0-cli/issues/546
[#597]: https://github.com/auth0/auth0-cli/issues/597
[#617]: https://github.com/auth0/auth0-cli/issues/617
[#618]: https://github.com/auth0/auth0-cli/issues/618
[#612]: https://github.com/auth0/auth0-cli/issues/612
[#595]: https://github.com/auth0/auth0-cli/issues/595

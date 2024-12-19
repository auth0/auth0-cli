# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

# [v.1.7.1](https://github.com/auth0/auth0-cli/tree/v1.7.1) (Dec 19, 2024))

[Full Changelog](https://github.com/auth0/auth0-cli/compare/v1.7.0...v1.7.1)

### Added

- Support flags for `auth0 ul customize` command to choose the renderingMode, prompt & screenNames along with configSettings file[#1111]

### Fixed

- Fix `auth0 tf generate` command and handle error when custom domain is not enabled [#1103]
- Fix CDN textLocal URL & include unit tests for fetchData of the resource `auth0_prompt_screen_renderer`[#1109]


# [v.1.7.0](https://github.com/auth0/auth0-cli/tree/v1.7.0) (Dec 9, 2024))

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

[unreleased]: https://github.com/auth0/auth0-cli/compare/v1.5.1...HEAD
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

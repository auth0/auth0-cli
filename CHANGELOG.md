# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0-beta.3](https://github.com/auth0/auth0-cli/tree/1.0.0-beta.3) (Mar 30, 2023)

Refer to the [v1 migration guide](MIGRATION_GUIDE.md) for instructions on how to navigate breaking changes.

To try the `v1.0.0-beta.3` release:

```bash
# Binary will be downloaded to "./auth0".
curl -sSfL https://raw.githubusercontent.com/auth0/auth0-cli/main/install.sh | sh -s -- -b . "v1.0.0-beta.3"

# Note: will only download executable in current directory.
# Intentionally omitted from $PATH to avoid collisions with
# stable versions of the CLI. Append to $PATH at own risk.

./auth0 --version # Example execution
```

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
- Possible panic when running `auth0 ul update` with empty branding settings (newly-created tenant) [#692]
- Inability to update user password with `auth0 users update --password` [#686]

## [1.0.0-beta.2](https://github.com/auth0/auth0-cli/tree/1.0.0-beta.2) (Feb 14, 2023)

### Added

- Storing access token in OS keyring if possible [#645]
- DX improvements to `test login` and `test token` commands [#629]

### Fixed

- Apps shown in multi select when no app-id is passed [#648]

## [1.0.0-beta.1](https://github.com/auth0/auth0-cli/tree/1.0.0-beta.1) (Jan 20, 2023)

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

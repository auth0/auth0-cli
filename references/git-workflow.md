# Git Workflow

## Branch naming

Observed conventions in this repo:

- Ticket-scoped: `DXCDT-1234/short-description` (Jira key + slug).
- Type-scoped: `docs/…`, `fix-…`, `issue-<number>-…`.
- Automated: `dependabot/…`.

Match the pattern that fits your change; prefer the ticket-scoped form when a Jira ticket exists.

## Commit messages

Conventional-commit style prefixes are used across history: `docs:`, `chore(deps):`, `fix:`, `feat:`. Keep the subject imperative and concise; scope in parentheses where useful (e.g. `chore(deps): bump ...`).

## Pull requests

Use `.github/PULL_REQUEST_TEMPLATE.md`, which has three sections:

- **🔧 Changes** — what changed and why; types/methods added, deleted, deprecated, or changed; usage summary for new/changed public surface.
- **📚 References** — GitHub issue/PR links, Community posts, related PRs.
- **🔬 Testing** — how the change was tested.

## Before opening a PR

1. `make lint`
2. `make test-unit`
3. `make docs` (if you touched commands/flags/help) — CI's `make check-docs` will fail otherwise.
4. `go mod tidy && go mod vendor` (if you touched dependencies).

## Releases

Releases are cut by maintainers via a tag-triggered GitHub workflow + Goreleaser — not by agents editing files. The `CHANGELOG.md` is written as part of that release flow (see the `Add changelog for vX.Y.Z` PRs), not by feature PRs. Do not bump versions or add changelog entries by hand as part of a feature change.

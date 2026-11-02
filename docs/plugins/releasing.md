---
layout: default
title: Releasing a plugin
parent: Plugins
nav_order: 4
---

In this page you publish a new version of a plugin as a GitHub release, with one archive per platform and a `checksums.txt` file. The release command and the release workflow of the starter project do the steps for you, so you never edit a version or a tag by hand.

## What the starter project gives you

Both starter projects contain a release command and two GitHub Actions workflows:

| Part | Go project | TypeScript project |
| --- | --- | --- |
| Release command | `make release` | `bun run release` |
| Program behind it | `scripts/release/` | `scripts/release.ts` and `scripts/version.ts` |
| Tests on every pull request | `.github/workflows/ci.yml` | `.github/workflows/ci.yml` |
| Release on merge | `.github/workflows/release.yml` | `.github/workflows/release.yml` |
| Builds the archives | GoReleaser, a tool that builds and packs Go release archives, configured in `.goreleaser.yaml` | `bun build --compile` in `.github/actions/build-archive/` |
| Reads the version and checks what exists | `.github/actions/read-version/` | `.github/actions/read-version/` |

The flow has two halves. A command on your machine opens a pull request that raises the version. When you merge that pull request, the workflow tests the commit, builds the archives and publishes the release.

## Create the first release

`plugin dev create` ends with this line:

```text
 ▸    To publish it, push the project to a new GitHub repository: the first push to main releases version 0.1.0. Release again with: make release BUMP=minor
```

The release workflow runs when a change to `plugin.json` reaches `main`, and a first push contains `plugin.json`. So the first push releases the version that the project was created with, once the tests pass.

A GitHub release does not list the plugin in the registry. Other people can install it with `auth0 plugin install` once it is listed in the registry catalog, which is a separate step. See [Status](index.md#status).

The workflows listen to `main`. If your default branch has another name, change the `branches` line in both `release.yml` and `ci.yml`.

## The version

The version lives in the `version` field of `plugin.json`. The tag is `v` followed by the version, and the archives carry the version in their names.

- Use `MAJOR.MINOR.PATCH`, without a leading `v`. This is the only form the CLI installs. If a registry lists a version such as `1.0.0-rc.1`, `auth0 plugin install` refuses it, because the version is not in the form `MAJOR.MINOR.PATCH`.
- The release command and the workflow accept a prerelease such as `1.0.0-rc.1` when you give the exact version, so you can publish one to GitHub for your own testing. They refuse build metadata such as `1.0.0+build.5`, because the tag and the archive names carry the version. The manifest schema accepts both forms.
- In the TypeScript project, `package.json` may have a `version` too. If it does, it must match `plugin.json`, and the release command keeps the two in step.

## Start a release

Run the command on a clean checkout of `main` that matches `origin`. It needs `git` and `gh`, and `gh` must be logged in (`gh auth login`).

For Go:

```sh
make release BUMP=patch              # or minor, or major
make release VERSION=1.2.3           # an exact version
make release BUMP=patch DRY_RUN=1    # show the steps, change nothing
```

For TypeScript:

```sh
bun run release patch              # or minor, or major
bun run release 1.2.3              # an exact version
bun run release patch --dry-run    # show the steps, change nothing
```

A dry run prints the plan and changes nothing. This is the output of the Go project, and the TypeScript project prints the same three lines:

```text
Releasing 0.1.1 (now 0.1.0) from main:
  create branch release/v0.1.1, set the version in plugin.json,
  commit "chore: release v0.1.1", push and open the pull request "Release v0.1.1"
Dry run: nothing was changed.
```

Without the dry run flag, the command does what the plan says. It creates the branch `release/v<version>`, raises the version in `plugin.json`, commits `chore: release v<version>` with your own git settings, pushes the branch, and opens a pull request titled `Release v<version>`. It then switches back to the default branch and prints the address of the pull request. The pull request is opened with your own login, so the repository does not need to allow Actions to create pull requests.

If a step fails before the push, the command restores the repository. It switches back to the default branch and deletes the release branch. If the branch is pushed but the pull request cannot be opened, the message tells you the command to open it yourself.

The command refuses to start in these cases. Both starter projects print the same messages:

```text
release: there are uncommitted changes. Commit or stash them first:
M README.md
```

```text
release: "bogus" is not patch, minor, major or a semantic version such as 1.2.3 (no build metadata).
```

```text
release: "v1.0.0" is not patch, minor, major or a semantic version such as 1.2.3 (no build metadata). Write 1.0.0 without the v.
```

It also refuses these cases:

- You are not on the default branch, or your checkout is not level with `origin`.
- A branch `release/v<version>` or a tag `v<version>` already exists, here or on `origin`.
- An exact version that is already the version in `plugin.json`.
- `patch`, `minor` or `major` while the current version is a prerelease. Then give the exact version.
- `gh` is not logged in. The message is `release: gh is not logged in. Run: gh auth login`.

## What the workflow does

Merging the pull request changes `plugin.json` on `main`, which starts the Release workflow. You can also start it by hand from the Actions tab, on `main`, to finish a release that stopped half way.

1. **Read the version.** The workflow reads `version` from `plugin.json`, derives the tag, and checks whether the tag and the release already exist. A release counts as complete when it holds `checksums.txt` and an archive for each platform in `plugin.json`. A manual run on a branch other than the default branch is skipped.
2. **Run the tests.** The workflow calls `ci.yml` on the merged commit. It skips this step when the release is already complete.
3. **Build the archives** for the seven platforms (see below).
4. **Write `checksums.txt`** with the SHA-256 hash of every archive.
5. **Create the release** `v<version>` at the merged commit, with the archives, `checksums.txt` and notes that GitHub generates. GitHub creates the tag at that commit.

The Go project does steps 3 to 5 with GoReleaser in one job. The TypeScript project builds each platform in its own job, with `bun build --compile --target=bun-<platform>`. Each job uploads its archive as a workflow artifact. A last job collects them, writes `checksums.txt` and publishes the release. In the TypeScript project, `checksums.txt` goes up last, because its presence is what marks a release as complete.

The workflow is safe to run again. If a run stops half way, run it again. It completes whatever is missing, and it does nothing once the release is complete. Two things protect you from publishing the wrong code:

- If the tag already exists at another commit and the release is not complete, the run stops with this message: `The tag v<version> is at <sha>, but this run is for <sha>. Raise the version in plugin.json, or re-run the workflow of the commit the tag points to.`
- Only one run per tag can publish at a time.

The workflow token has read access to the repository. Only the job that publishes also gets write access to the contents, so it can create the tag, the release and the assets. The actions in the workflows are pinned to full commit hashes.

## What an archive must contain

For each platform in `plugin.json`, the release has one archive named `<name>_<version>_<platform>.tar.gz`, or `.zip` for Windows. Each archive holds exactly two files at its root:

- the executable, named `<name>`, or `<name>.exe` on Windows,
- `plugin.json`.

Nothing else is allowed. When someone installs the plugin, the CLI refuses an archive that has directories, links, other file names, duplicate entries, or an executable whose name is not the one the registry lists. That is why the starter projects leave out the README and the license.

This is what the Go project produced with `make release-snapshot`, which builds every archive into `dist/` without publishing anything. The middle part of each name is the snapshot version, which depends on the git state of the project:

```text
dist/checkmate_0.0.0-SNAPSHOT-none_darwin-arm64.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_darwin-x64.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_linux-arm64-musl.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_linux-arm64.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_linux-x64-musl.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_linux-x64.tar.gz
dist/checkmate_0.0.0-SNAPSHOT-none_windows-x64.zip
dist/checksums.txt
```

The contents of two of them:

```text
$ tar -tzf checkmate_0.0.0-SNAPSHOT-none_linux-x64.tar.gz
plugin.json
checkmate

$ unzip -l checkmate_0.0.0-SNAPSHOT-none_windows-x64.zip
  Length      Date    Time    Name
---------  ---------- -----   ----
      894  10-30-2026 08:49   plugin.json
  8444416  10-30-2026 08:51   checkmate.exe
---------                     -------
  8445310                     2 files
```

For the TypeScript project, the script that the workflow runs for one platform also works on your machine. It needs `bash`, `jq`, Bun, and `zip` for Windows archives:

```sh
bash .github/scripts/build-archive.sh linux-x64-musl
```

```text
out/checkmate_0.1.0_linux-x64-musl.tar.gz
```

That archive holds `checkmate` and `plugin.json`.

### Platforms

`plugin.json` lists the platforms you ship. The starter projects ship all seven. A release must have an archive for every platform in the manifest.

| Platform | Go build | Bun target |
| --- | --- | --- |
| `darwin-arm64` | `darwin/arm64` | `bun-darwin-arm64` |
| `darwin-x64` | `darwin/amd64` | `bun-darwin-x64` |
| `linux-x64` | `linux/amd64` | `bun-linux-x64` |
| `linux-arm64` | `linux/arm64` | `bun-linux-arm64` |
| `linux-x64-musl` | `linux/amd64`, built again as its own archive | `bun-linux-x64-musl` |
| `linux-arm64-musl` | `linux/arm64`, built again as its own archive | `bun-linux-arm64-musl` |
| `windows-x64` | `windows/amd64` | `bun-windows-x64` |

The Go project builds a static binary with `CGO_ENABLED=0`, so the musl archives contain the same program as the glibc ones. In the snapshot, the `linux-x64` and `linux-x64-musl` archives have the same hash for that reason. On Linux, the CLI checks which C library the machine uses and picks the matching platform.

### `checksums.txt`

`checksums.txt` has one line per archive, in the format of `sha256sum`: the hash, two spaces and the file name. Two lines from the same snapshot:

```text
36e4c1e39e3285938cdd7aaeceefb5b5ef97bbcc52b0893ed38b55fda182a10b  checkmate_0.0.0-SNAPSHOT-none_darwin-arm64.tar.gz
cd7564598778ea839e5a32bca14912bc046471c9e7655997e14fc82c4b1be931  checkmate_0.0.0-SNAPSHOT-none_darwin-x64.tar.gz
```

The CLI does not read this file. The hash that the CLI checks when a person installs a plugin comes from the signed registry index. Use `checksums.txt` to compare an archive that you downloaded with the one that the workflow built.

## What is automated, and what is not

The starter project automates these steps:

- Opening the release pull request from your machine, with the version raised and a standard commit message.
- Running the tests on the merged commit.
- Creating the tag `v<version>` at that commit.
- Building the archives for seven platforms, with the files that the CLI accepts.
- Writing `checksums.txt`.
- Creating the GitHub release with generated notes.
- Finishing a release that stopped half way, and doing nothing once it is complete.

## What the starter does not do

- Sign the archives or record a build attestation. The workflows contain no signing step and do not ask for an attestation.
- Turn on immutable releases. This is a repository setting on GitHub. The starter project neither turns it on nor checks it.
- List the plugin in the registry catalog. No command or workflow in the starter project does this.

## Next

- [Reference](reference.md) lists the fields of `plugin.json`, the platform names, the paths and the reason codes.
- [Security and trust model](security.md) explains how the CLI checks what it installs.
- [Building a plugin](building.md) covers the work before a release.

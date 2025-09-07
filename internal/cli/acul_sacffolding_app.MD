# Scaffolding Approaches: Comparison and Trade-offs

## Method A: Git Sparse-Checkout
*File: `internal/cli/acul_scaff.go` (command `init1`)*

**Summary:**  
Initializes a git repo in the target directory, enables sparse-checkout, writes desired paths, and pulls from branch `monorepo-sample`.

**Pros:**
- Efficient for large repos; downloads only needed paths.
- Preserves git-tracked file modes and line endings.
- Simple incremental updates (pull/merge) are possible.
- Works with private repos once user’s git is authenticated.

**Cons:**
- Requires `git` installed and a relatively recent version for sparse-checkout.


---

## Method B: HTTP Raw + GitHub Tree API
*File: `internal/cli/acul_scaffolding.go` (command `init`)*

**Summary:**  
Uses the GitHub Tree API to enumerate files and `raw.githubusercontent.com` to download each file individually to a target folder.

**Pros:**
- No git dependency; pure HTTP.
- Fine-grained control over exactly which files to fetch.
- Easier sandboxing; fewer environment assumptions.

**Cons:**
- Many HTTP requests; slower and susceptible to GitHub API rate limits.
- Loses executable bits and some metadata unless explicitly restored.
- Takes more time to download many small files [so, removed in favor of Method C].


---

## Method C: Zip Download + Selective Copy
*File: `internal/cli/acul_sca.go` (command `init2`)*

**Summary:**  
Downloads a branch zip archive once, unzips to a temp directory, then copies only base directories/files and selected screens into the target directory.

**Pros:**
- Single network transfer; fast and API-rate-limit friendly.
- No git dependency; works in minimal environments.
- Simple to reason about and easy to clean up.
- Good for reproducible scaffolds at a specific ref (if pinned).

**Cons:**
- Requires extra disk for the zip and the unzipped tree.
- Tightly coupled to the zip’s top-level folder name prefix. 
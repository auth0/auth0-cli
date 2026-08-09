# Commands

All commands are Makefile targets (run `make help` to list them). These mirror what CI runs in `.github/workflows/main.yml`.

```bash
# Build the CLI binary for the native platform -> ./out/auth0
make build

# Install the binary into $GOPATH/bin
make install

# Build for all supported platforms (CI "Build" job)
make build-all-platforms

# Run unit tests (safe — no credentials required)
make test-unit

# Run all tests (unit + integration; integration needs a live tenant)
make test

# Run integration tests only (requires AUTH0_DOMAIN/CLIENT_ID/CLIENT_SECRET)
make test-integration
# Filter to a subset:
make test-integration FILTER="attack protection"

# Regenerate gomock mocks (after changing a mocked interface)
make test-mocks

# Lint (golangci-lint v2, config .golangci.yml)
make lint

# Check for known vulnerabilities (govulncheck)
make check-vuln

# Regenerate the docs/ command reference from Cobra commands
make docs

# Verify docs are in sync (CI gate — fails if `make docs` produces a diff)
make check-docs

# Download dependencies
make deps

# Clean the docs output
make docs-clean
```

## CI jobs (`.github/workflows/main.yml`)

1. **Checks** — `make check-docs` + `golangci-lint` with `-c .golangci.yml`.
2. **Unit Tests** — `make test-unit`, uploads coverage.
3. **Integration Tests** — `make test-integration` (skipped for forks/dependabot; only on `main`-targeted PRs). Uses `AUTH0_DOMAIN`, `AUTH0_CLIENT_ID`, `AUTH0_CLIENT_SECRET` secrets.
4. **Build** — `make build-all-platforms`.

## Running without building

```bash
go run ./cmd/auth0 <command>
```

# Testing

## Frameworks & layout

- **Unit tests:** Go's standard `testing`, with `github.com/stretchr/testify` (`assert`) for assertions and `github.com/golang/mock/gomock` for mocks.
- **Location:** colocated `*_test.go` files next to the code (e.g. `internal/cli/apps_test.go`), same package.
- **Integration tests:** YAML-driven test cases under `test/integration/*-test-cases.yaml`, executed by `commander` against a live Auth0 tenant.
- **Coverage:** produced by `make test-unit` (`coverage-unit-tests.out`) and uploaded to Codecov in CI. `codecov.yml` holds the config; there is no hard local threshold gate.

## Running tests

```bash
# All unit tests (safe — no credentials)
make test-unit

# A single package
go test -race ./internal/cli/...

# A single test by name
go test -race ./internal/cli/ -run TestAppsListCmd

# Integration subset by filter
make test-integration FILTER="apps"
```

## Unit test conventions

- **Table-driven:** a `tests := []struct{ name string; args []string; assertOutput func(...) }` slice iterated with `t.Run(tt.name, ...)`.
- **Mocking the API:** use the generated mocks in `internal/auth0/mock` with a `gomock.Controller`; set expectations via `EXPECT()`. Regenerate with `make test-mocks` after changing a mocked interface.
- **Output assertions:** command output is captured and checked with helpers like `expectTable(t, out, headers, rows)`.
- Name cases descriptively ("happy path", "reveal secrets") rather than by index.

## Integration / Acceptance tests

> ⚠️ These hit a **live Auth0 tenant**, are slow, and can create/modify/delete real resources. **Ask before running them** (see Boundaries in CLAUDE.md).

```bash
# Requires: AUTH0_DOMAIN, AUTH0_CLIENT_ID, AUTH0_CLIENT_SECRET
# (set in a .env at repo root or exported)
make test-integration
```

Get these values by setting up a Machine-to-Machine application in your Auth0 tenant (see `CONTRIBUTING.md`).

# Docs Update Rules

This is a **CLI** repo, so the code-to-docs mapping is command/flag-oriented.

## Tracked docs

| Doc | Covers | Present |
|-----|--------|---------|
| `README.md` | Install, quickstart, top-level command overview | present |
| `docs/*.md` | **Generated** per-command reference (`make docs`) — one file per command | present |
| `CUSTOMIZATION_GUIDE.md` | Universal Login / branding customization workflow | present |
| `MIGRATION_GUIDE.md` | Migration notes for breaking changes | present |
| `CONTRIBUTING.md` | Dev setup, adding a command, adding a dependency, releasing | present |

> `EXAMPLES.md` is not tracked in this repo — usage examples live in `README.md` and the generated `docs/`.

## When you change code, update these docs

| Change | Update |
|--------|--------|
| Add/rename/remove a command | `make docs` (regenerates `docs/`), plus `README.md` if it's a top-level command |
| Add/change a flag or its help text | `make docs` |
| Change command output/behavior | `make docs` if help text changed |
| Breaking change (flag/output/command removed or renamed) | Ask first; then `MIGRATION_GUIDE.md` + `make docs` |
| Change to Universal Login / branding flow | `CUSTOMIZATION_GUIDE.md` |
| Change dev setup, build, or release steps | `CONTRIBUTING.md` |

> The generated `docs/` reference must never be hand-edited — always regenerate via `make docs`. CI's `make check-docs` enforces this. Update the mapped hand-written doc **in the same PR** as the code change.

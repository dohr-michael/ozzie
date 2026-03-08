---
description: Go code quality gates — applies to all Go file changes
globs: "**/*.go"
---

# Go Quality Gates

Every change MUST pass all three gates before being considered done:

1. **Compile**: `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build ./...`
2. **Lint**: `staticcheck ./...` (installed at `~/go/bin/staticcheck`)
3. **Test**: `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...`

> `CGO_CFLAGS` is required for SQLite FTS5 (used by `pkg/memory/`). Without it, FTS5 virtual tables fail at runtime.

These are equally important. A staticcheck warning (especially SA1019 deprecated, SA4009 unused) is a build failure.

## Style

- Follow standard Go conventions (gofmt, effective Go)
- No `//nolint` or `staticcheck:ignore` without explicit justification
- Prefer `internal/` packages — nothing is exported at the module root
- Error wrapping: use `fmt.Errorf("context: %w", err)` consistently

# Contributing to engram

## Getting started

```bash
git clone <repo-url>
cd engram
make build
make test
```

Requires Go 1.22+ and a C compiler (for SQLite).

## Code standards

- Go standard formatting (`gofmt`)
- All exported functions have a comment
- No dependencies beyond `BurntSushi/toml` and `mattn/go-sqlite3`
- All SQL queries use parameterized statements (no string interpolation)
- File permissions: database 0600, config 0640, directories 0750

## Testing

```bash
make test          # run all tests
make bench         # run benchmarks
```

Tests use in-memory SQLite databases and require no external setup.

## Commits

Use conventional commit messages:

```
feat: add tag-based synonym expansion
fix: handle empty query in search fallback
docs: update benchmark results in README
test: add edge case for FTS5 with single-word query
```

## Pull requests

- One logical change per PR
- Include tests for new functionality
- Run `go vet ./...` before submitting
- Benchmark any changes to the hot path (`make bench`)

## Architecture principles

- **No servers.** engram is a CLI tool. No daemons, no background processes.
- **Two dependencies.** Do not add new external dependencies without discussion.
- **Fast.** Every command should complete in under 50ms. The hot path (`get`) should be under 5ms for 10K corrections.
- **Simple.** Prefer 10 lines of clear code over 3 lines of clever code.

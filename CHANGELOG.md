# Changelog

## 0.2.0 (2026-04-12)

### Changed
- Replaced cobra CLI framework with stdlib flag parsing (removed 4 dependencies)
- Binary size reduced from 7.3MB to 4.8MB
- Removed daemon, MCP server, and OpenAI proxy (CLI-only architecture)
- Default `min_score` changed from 0.1 to 0.0 for better small-corpus retrieval

### Added
- `engram store` command for direct CLI correction storage
- `engram get` command with `--all` and `--raw` flags for flexible retrieval
- LIKE-based fallback search when FTS5/BM25 returns no results
- `engram init --hooks` to auto-install Claude Code slash commands and prompt hook
- Claude Code slash commands: `/remember`, `/forget`, `/recall`, `/corrections`
- Global `--db` flag to skip config loading for maximum speed
- `--version` flag
- Rich tag support for improved retrieval quality
- Makefile with build, test, bench, install targets
- Benchmark test suite
- Security hardening: restrictive file permissions on database and config

### Removed
- `engram daemon` command (MCP server)
- `internal/mcp/` package
- `internal/proxy/` package (OpenAI proxy)
- Dependencies: `mark3labs/mcp-go`, `google/uuid`, `spf13/cobra`, `spf13/pflag`

## 0.1.0 (2026-04-12)

### Added
- Initial implementation with MCP server architecture
- SQLite storage with FTS5 full-text search
- BM25-ranked retrieval
- Project detection via `.engram` marker files
- CLI commands: init, list, search, delete, edit, stats, export, import
- OpenAI-compatible proxy mode

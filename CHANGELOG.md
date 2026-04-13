# Changelog

## 0.5.0 (unreleased)

### Added
- `engram mcp` command. Starts a stdio MCP server exposing three tools:
  `store`, `search`, and `get`. Works with any MCP-aware host (Cursor,
  Windsurf, Claude Desktop, etc.). No SDK, no daemon, no new dependencies —
  pure JSON-RPC 2.0 over stdin/stdout.
- Passive correction capture via `PostToolUse` hook. When `engram hook`
  detects a correction pattern but the LLM doesn't call `engram store` before
  its next Bash invocation, `engram capture` injects a reminder so the
  correction isn't silently lost.
- `engram init --hooks` now also installs the `PostToolUse` hook for passive
  capture. Existing projects should re-run `engram init --hooks` to activate.

### Changed
- `engram init --project` prints MCP config hint after hook installation.
- Version bumped to 0.5.0.

## 0.4.0 (2026-04-13)

### Retrieval accuracy
- Hook now uses the user's prompt as the search query instead of returning
  scope-filtered corrections in arbitrary order. The biggest single
  accuracy improvement in the release.
- New code-friendly FTS5 tokenizer keeps identifiers like `burntsushi/toml`,
  `use_state`, and `github.com/spf13/cobra` as single tokens.
- New trigram secondary index handles typos and partial matches that the
  primary tokenizer misses.
- Negation words (`not`, `no`) are no longer stripped from search queries —
  corrections about what *not* to do now retrieve correctly.
- Superseded corrections are filtered out at retrieval time. Previously,
  superseding a correction left the old version in the injection pool.
- Project-scoped corrections now soft-preferred over globals via a tier
  bonus rather than hard-partitioned. A strongly-matching domain or global
  correction can still win.
- MMR diversity in selection prevents the memory block from filling up with
  near-duplicate corrections.
- Recency decay no longer floors at 20% — half-life extended to 365 days
  with a 5% floor.

### Storage
- Dedup check on `engram store`: similar corrections in the same scope are
  flagged with guidance to use `--supersedes` or `--force`. Use `--force`
  to bypass.

### Performance
- Scope filtering pushed into SQL instead of in-Go post-filtering. Hooks
  no longer load the entire corrections table every turn.
- BM25 minimum score threshold pushed into SQL WHERE clause.

### Internal
- Migration runner generalized to apply all `schema/NNN_*.sql` files in
  order. Schema version is now 3.
- Latent bug fixed in hook scope filter loop.

## 0.3.0 (2026-04-13)

### Changed
- **Single-command project setup.** `engram init --project` now installs the `.engram` marker, Claude Code prompt hook, and all slash commands in one step. No separate `engram init --hooks` needed. Database auto-creates on first use.
- **`engram hook` command replaces `engram get --all` in the prompt hook.** Reads the user's prompt from Claude Code's stdin JSON and pattern-matches for correction indicators ("actually," "that's wrong," "no, it's X not Y," "remember that," etc.). When detected, injects a contextual `⚠️ CORRECTION DETECTED` alert that directs the LLM to store the correction immediately — far more reliable than generic instructions alone.
- **`engram init --project` writes CLAUDE.md instructions.** Asks the user to create (or append to) CLAUDE.md with engram behavior instructions. CLAUDE.md is the most authoritative instruction source for Claude Code — hook-injected instructions alone were not reliably followed.
- **Visible acknowledgment on store.** The LLM now confirms corrections with `▣ Stored in engram memory: <summary>` instead of operating silently. Gives users confidence engram is working.
- **CLI command references in behavior prompt.** Replaced abstract function names (`store_correction`, `get_corrections`) with actual CLI syntax (`engram store`, `engram get`, `engram list`).
- **Schema: clean break.** New canonical schema with `type`, `trigger_hint`, and `supersedes_id` columns. Existing databases must be recreated (no migration from 0.2.0).
- **Retrieval: phrase-first cascade.** Search now tries exact phrase match first, then AND (all terms), then OR (any term), then LIKE fallback. More precise results with no performance cost — the phrase tier short-circuits early.
- **BM25 column weights tuned.** `fact`x10, `wrong`x1, `tags`x5, `trigger_hint`x3. Fact matches now properly dominate over incidental matches in the wrong field.
- **LIKE fallback uses AND logic.** Previously OR — a query like "config toml" would match anything with "config" OR "toml". Now requires all non-stop-word terms to appear.
- **Composite scoring replaces raw BM25 for selection.** Combines BM25 relevance, hit frequency (log-scale), recency (180-day decay half-life), and confidence. Frequently-used corrections rank higher; stale ones decay but never fully disappear.
- **Scope priority in selection.** Corrections are now ordered: global first, then current project, then domain/other. Each tier sorted by composite score.
- **Memory block grouped by type.** Injected corrections are now labeled by type (CONSTRAINTS first, then GOTCHAS, FACTS, PROCESS, PREFERENCES) so the LLM immediately sees which entries are inviolable.
- **Scope clustering.** When 3+ corrections share the same scope, they're grouped under a single scope header instead of repeating the prefix on every line. Saves tokens.
- **Pragmas folded into DSN.** Eliminated `setPragmas()` method. Pragmas set at connection open via query string, one fewer round-trip.
- **Adaptive mmap_size.** Maps 4x the actual DB file size (capped at 512MB, floor 32MB) instead of a fixed 128MB.
- **64MB page cache.** `cache_size=-65536` for faster repeated queries within a single invocation.
- **Connection pool tuned for SQLite.** `MaxOpenConns(1)`, `MaxIdleConns(1)`, `ConnMaxLifetime(0)` — prevents spurious extra connections while keeping the single connection warm.
- **Hit count update is async.** Output prints first, then hit counts update in a background goroutine (with WaitGroup for clean shutdown).
- **Export/import includes new fields.** JSON and TOML export now includes type, trigger_hint, and supersedes_id.
- **`/remember` slash command rewritten.** Now includes pre-store dedup check, type selection guidance, trigger_hint writing instructions, and supersedes support.
- **`/recall` slash command updated.** Explicit guidance on constructing specific queries from the current topic.

### Added
- **`engram hook` command.** Purpose-built hook handler for Claude Code's `UserPromptSubmit` event. Reads the user's prompt via stdin JSON, outputs behavior instructions and corrections, and detects correction patterns to trigger immediate storage. Replaces the raw `engram get --all` approach.
- **Correction detection via pattern matching.** 20+ regex patterns covering common correction phrases: explicit corrections ("actually," "that's wrong"), preference statements ("remember that," "going forward"), and frustration signals ("I told you," "how many times"). No NLP or network calls — pure regex, runs in under 1ms.
- **CLAUDE.md auto-installation.** `engram init --project` prompts to create or append a `<!-- engram:start -->` section to CLAUDE.md with mandatory behavior instructions. Skips if the section already exists. User can decline (with a warning that auto-store may not work).
- **Correction types.** Each correction is now one of: `fact`, `preference`, `constraint`, `gotcha`, `process`. Enforced by a CHECK constraint in SQLite.
- **Trigger hints.** A `trigger_hint` field describes *when* a correction should surface ("when writing config loading code"). Indexed by FTS5 and contributes to search with weight 3.
- **Supersedes tracking.** `--supersedes <id>` links a new correction to the one it replaces. Foreign key back to the corrections table.
- **`engram store --type`** flag for setting correction type (default: `fact`).
- **`engram store --trigger`** flag for setting the trigger hint.
- **`engram store --supersedes`** flag for linking to a replaced correction.
- **`engram vacuum`** command. Runs incremental vacuum, `PRAGMA optimize`, and rebuilds the FTS5 index.
- **`engram list --stale`** flag. Shows corrections with zero hits or not retrieved in 180+ days.
- **`engram stats` type breakdown.** Shows count per correction type.
- **`engram stats` stale count.** Reports how many corrections haven't been retrieved in 180 days.
- **`engram edit` exposes new fields.** Type and trigger_hint are now editable in the JSON editor.
- **`UpdateFields` expanded.** `Type` and `TriggerHint` can be updated via the `Update()` method.
- **`Int64` flag type in Args parser.** Supports `--supersedes` and future int64 flags.
- **Stop word filtering.** 30 common English words are filtered from search queries to reduce noise.
- **Database indexes.** `idx_corrections_scope`, `idx_corrections_type`, `idx_corrections_supersedes` for faster filtered queries.
- **12 new tests.** Schema columns, supersedes, type validation, stale list, phrase cascade, stop words, trigger_hint search, column weight ranking, constraint rendering order, scope clustering, composite scoring, project scope priority.
- **2 new benchmarks.** Phrase-first cascade at 1K and 10K corpus sizes.

### Fixed
- **Empty database suppressed behavior prompt.** When no corrections existed yet, `engram get --all` returned nothing — the LLM never received instructions on how to use engram. Now always outputs the behavior prompt.

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

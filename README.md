# engram

![logo](assets/logo.png)

Correction memory for LLMs.

Every time you correct an AI — "that function doesn't exist," "my project uses X not Y," "stop formatting in markdown" — that correction vanishes when the conversation ends. Next session, the same mistake. engram fixes this.

It stores corrections as atomic facts in a local SQLite database, retrieves relevant ones via BM25 full-text search, and injects them into future AI sessions. The AI learns from corrections permanently, not just for one conversation.

## How it works

```
You correct the AI  →  engram stores the fact  →  next session, engram injects it  →  mistake never repeats
```

engram is a single CLI binary. No daemon, no server, no cloud. It integrates with Claude Code via slash commands and a hook that runs automatically on every prompt. The AI calls `engram store` when it detects corrections, and the hook calls `engram get` to retrieve relevant ones. You never interact with engram directly — you just talk to the AI.

### How is this different from MemPalace?

[MemPalace](https://github.com/MemPalace/mempalace) is a full conversation memory system — it remembers *everything* from past sessions using vector embeddings and a knowledge graph. It's designed for total recall across long histories. engram is a correction memory — it remembers only what you got *wrong*, and it's built to do that one thing as fast as possible. MemPalace needs ChromaDB, Python, and an embedding model. engram is a single 4.8MB binary with a 3ms query time. Use MemPalace if you want an AI that remembers entire conversations. Use engram if you want an AI that stops repeating the same mistakes.

### How is this different from Claude Code's built-in memory?

Claude Code has its own memory system (markdown files in `~/.claude/projects/`). It's general-purpose context that the LLM reads when it thinks it's relevant. engram is purpose-built for corrections with structured retrieval.

| | Claude Code memory | engram |
|---|---|---|
| Storage | Markdown files | SQLite + FTS5 |
| Retrieval | LLM reads files it thinks are relevant | BM25 ranked search, scoped |
| Scope | Per-project directory path | global / project / domain |
| Search | LLM judgment | Full-text search with ranking |
| Structure | Freeform markdown | Atomic facts with tags, scope, wrong field, hit tracking |
| Focus | General — user prefs, project context, references | Specific — corrections that prevent repeated mistakes |

They're complementary. Claude Code memory remembers "this user prefers terse responses." engram remembers "this project uses toml not viper" and retrieves it when config parsing comes up.

## Install

```bash
# Clone and build
git clone <repo-url>
cd engram
make build

# Install to /usr/local/bin (may require sudo)
sudo make install

# Or install manually
sudo cp engram /usr/local/bin/
```

## Setup

Three commands. You do this once.

```bash
# 1. Create config and database
engram init

# 2. Mark your project for project-scoped memory (optional, per-project)
cd ~/projects/myproject
engram init --project

# 3. Install Claude Code slash commands and hook
engram init --hooks
```

That's it. From now on, corrections accumulate automatically.

### What `init --hooks` installs

**Slash commands** — available in Claude Code as `/remember`, `/forget`, `/recall`, `/corrections`:

| Command | What it does |
|---|---|
| `/remember` | Store a correction, preference, or fact |
| `/forget` | Find and delete a stored correction |
| `/recall` | Retrieve relevant corrections for the current topic |
| `/corrections` | List, search, export, and manage all corrections |

**Prompt hook** — automatically injects relevant corrections at the start of every prompt, so the AI has context before it responds.

## What it looks like in practice

You're working on a project and the AI suggests using viper for config parsing:

```
You:  We don't use viper in this project, we use BurntSushi/toml.
AI:   Got it, here's the corrected version...
```

Behind the scenes, the AI silently runs:
```bash
engram store "This project uses BurntSushi/toml for config, not viper." --scope project:myproject --wrong "viper"
```

Next session, before you even say anything, the hook injects:
```
[project:myproject] This project uses BurntSushi/toml for config, not viper.
```

The AI never makes that mistake again.

## Benchmarks

### CLI wall time (full round-trip)

Total time from process launch to output, including Go runtime startup, config loading, SQLite open, query, and output. Measured with `time` on an AMD Ryzen 7 7735HS.

| Corpus size | `store` | `get` (query) | `get --all` | `search` | `list` |
|---|---|---|---|---|---|
| 10 | 3ms | 3ms | 3ms | 3ms | 3ms |
| 100 | 3ms | 4ms | 4ms | 4ms | 3ms |
| 1,000 | 3ms | 5ms | 7ms | 5ms | 3ms |
| 5,000 | 3ms | 12ms | 24ms | 14ms | 5ms |

### Internal benchmarks (no process startup)

Pure query time measured by Go's `testing.B` framework:

| Corpus size | BM25 search | BM25 + scope filter | Store | List |
|---|---|---|---|---|
| 10 | 0.16ms | - | 0.06ms | - |
| 100 | 0.25ms | - | 0.06ms | - |
| 1,000 | 0.88ms | - | 0.06ms | 0.27ms |
| 10,000 | 5.5ms | 4.0ms | 0.06ms | - |

### What this means

- **Store is constant-time** regardless of corpus size (single INSERT + FTS trigger)
- **Search scales linearly** with corpus but stays under 15ms even at 5,000 corrections
- **The hook** runs `get --all` on every prompt — at typical corpus sizes (10-500 corrections), that's 3-4ms. Invisible.
- **No embedding inference on the hot path.** The entire retrieval is BM25 over SQLite FTS5. Compare to vector-DB systems that need 50-200ms per query for embedding inference alone.

Run benchmarks yourself: `make bench`

## Scopes

Corrections are scoped so the right facts appear in the right context:

- **global** — preferences and facts that apply everywhere ("prefer prose over bullet points")
- **project:name** — facts about a specific codebase ("this project uses age for encryption")
- **domain:tag** — facts about a technology ("Go 1.22 doesn't support range over integers")

engram auto-detects projects by walking up the directory tree looking for a `.engram` marker file (created by `engram init --project`). Commit this file to share project-scoped memory with your team.

## CLI reference

```
engram init                  Initialize config and database
engram init --project        Create .engram project marker in current directory
engram init --hooks          Install Claude Code slash commands and prompt hook

engram store <fact>          Store a correction
  --scope <scope>              global, project:<name>, domain:<tag> (default: auto-detect)
  --wrong "<text>"             What was previously assumed incorrectly
  --tags "<comma,separated>"   Tags for categorization
  --source <user|inferred>     How the correction originated

engram get [query]           Retrieve relevant corrections
  --all                        Return all corrections for current scope
  --raw                        Plain text output (one per line)
  --scope <scope>              Filter by scope
  --limit <n>                  Max corrections returned

engram list                  List all corrections
  --scope <scope>              Filter by scope
  --tag <tag>                  Filter by tag
  --limit <n>                  Max results

engram search <query>        Search with BM25 relevance scores
engram delete <id>           Delete a correction
engram edit <id>             Edit a correction in $EDITOR
engram stats                 Usage statistics and hit counts
engram export                Export as JSON or TOML
engram import <file>         Import from JSON or TOML

Global flag:
  --db <path>                Skip config loading, use database directly
```

## Storage

Everything lives in a single SQLite file at `~/.local/share/engram/engram.db`. Back it up with `cp`. Inspect it with `sqlite3`. Move it to a new machine by copying the file.

Retrieval uses SQLite FTS5 with BM25 ranking, with automatic LIKE fallback for edge cases. No embedding models, no vector databases, no network calls.

## Config

`~/.config/engram/config.toml`:

```toml
[database]
path = "~/.local/share/engram/engram.db"

[injection]
max_corrections = 10    # max corrections per retrieval
max_tokens      = 300   # token budget for injection block
min_score       = 0.0   # minimum BM25 relevance score

[log]
level = "warn"
```

## Design principles

- **One binary.** No Docker, no runtime, no Node. `make build` and done. 4.8MB.
- **Two dependencies.** SQLite and TOML. That's it. No frameworks.
- **No server.** No daemon, no MCP, no proxy. A CLI that reads and writes a SQLite file.
- **Fast.** 3ms total wall time. Sub-5ms search on 10,000 corrections.
- **Private.** Everything local. No telemetry. No cloud.
- **Invisible.** After setup, you never think about engram. You just talk to the AI.

## License

[MIT](LICENSE)

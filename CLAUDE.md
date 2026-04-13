# engram

> Correction memory for LLMs. Stores corrections and clarifications made during AI conversations and injects relevant ones into future context — preventing repeated hallucinations and eliminating wasted tokens on wrong assumptions.

---

## What this is

When you correct an LLM — "that function doesn't exist," "my project uses X not Y," "stop formatting in markdown" — that correction vanishes the moment the conversation ends. Next session, the model makes the same mistake. `engram` fixes this.

It's a single CLI binary backed by SQLite. The LLM calls `engram store` and `engram get` via Bash during conversations. A Claude Code hook runs `engram get` automatically at the start of every prompt, injecting relevant corrections into context. The user never has to think about engram during normal use — they just talk to the LLM and corrections accumulate.

---

## Architecture

engram is a CLI tool. No daemon, no server, no MCP protocol, no proxy. One binary, one database file.

```
engram store "fact"  -->  SQLite (FTS5 + BM25)  -->  engram get "query"  -->  <engram-memory> block
```

Integration with Claude Code happens through a hook that runs `engram get` on every prompt submission, prepending relevant corrections to context.

---

## User experience: transparent by design

The primary design constraint is that engram must be **invisible during normal use**. Setup happens once. After that, the user interacts only with the LLM in natural language.

### Setup (once per machine, once per project)

```bash
# Install
make build && sudo make install
# Or install manually: sudo cp engram /usr/local/bin/

# Enable engram in a project — one command, that's it
cd ~/projects/myproject
engram init --project
```

That's the entire setup. `engram init --project` creates the `.engram` marker file, installs the Claude Code prompt hook, and adds `/remember`, `/forget`, `/recall`, and `/corrections` slash commands. The database is created automatically on first use — no separate init step required.

You can also run `engram init` separately to explicitly create the global config and database, or `engram init --hooks` to reinstall just the Claude Code integration.

### Normal use — no commands required

After setup, the user just has conversations. Engram handles everything:

**Corrections are captured automatically:**

```
User:  Write a function to read the config file.
AI:    [writes code using viper]
User:  We don't use viper in this project, we use BurntSushi/toml.
AI:    ▣ Stored in engram memory: project uses BurntSushi/toml, not viper.
       Here's the corrected version... [calls: engram store "This project uses BurntSushi/toml for config, not viper." --scope project:myproject --wrong "viper"]
```

Next session, engram injects: `[project:myproject] This project uses BurntSushi/toml for config, not viper.`

**The user can speak directly to engram in plain English:**

```
User:  Remember that the dev server requires port 8080.
User:  Forget what you know about the old auth flow.
User:  What have you remembered about this project?
```

The LLM interprets these, calls the appropriate engram CLI command, and responds naturally.

### What the user never has to do

- Run `engram store` or `engram add` during a conversation
- Explicitly tell the LLM to remember something (though they can if they want)
- Think about scopes or tags during normal use (the LLM infers them)
- Start or stop any background process

---

## Core design principles

- **Single binary.** No Docker, no Python runtime, no Node. One `engram` binary. Ships via `go install` or a release tarball.
- **No server.** No daemon, no MCP protocol, no proxy. Just a CLI that reads and writes a SQLite database.
- **Transparent.** After `engram init`, the user interacts only with the LLM. Engram is invisible during normal use.
- **Zero heavy dependencies.** No embedding models, no vector databases, no network calls for retrieval. BM25 + FTS5 only.
- **SQLite-backed.** All data lives in one `.db` file. Portable, inspectable, trivially backed up with `cp`.
- **Compact storage.** Corrections are atomic facts, not conversation logs. Each entry is small by design.
- **Retrieval in under 5ms.** SQLite FTS5 with BM25 ranking. No embedding inference on the hot path.
- **Privacy-first.** Everything local. No telemetry. No cloud sync.
- **Project-aware.** Detects `.engram` marker files walking up the directory tree and automatically scopes memory.

---

## Tech stack

| Layer | Choice | Reason |
|---|---|---|
| Language | Go 1.22+ | Single binary, fast startup, good stdlib |
| Storage | SQLite with FTS5 via mattn/go-sqlite3 | Built-in full-text search, BM25 ranking, zero server |
| CLI | stdlib flag + manual dispatch | Zero framework overhead, two direct dependencies total |
| Config | TOML via BurntSushi/toml | Simple, human-editable |

**Dependency rule:** Only two direct dependencies: `mattn/go-sqlite3` (SQLite) and `BurntSushi/toml` (config parsing). Everything else uses the Go standard library.

---

## Project detection

When engram runs, it walks up the directory tree from the working directory looking for a `.engram` file. This mirrors how git finds `.git`.

```
/home/user/projects/myproject/internal/db/  -> not found
/home/user/projects/myproject/internal/     -> not found
/home/user/projects/myproject/              -> found .engram -> project scope = "myproject"
```

The discovered project name is used to scope corrections. If no `.engram` is found, scope defaults to `global`.

The `.engram` file is a simple TOML marker:

```toml
# .engram - project scope marker for engram
project = "myproject"
```

---

## LLM behavior specification

This section defines how the LLM must behave when engram is available. These instructions are what makes engram work — the LLM must follow them.

### How the LLM uses engram

The LLM calls engram via Bash. It never announces these operations to the user.

**Storing corrections** — call whenever:
- The user explicitly corrects something ("that's wrong," "actually," "no,")
- The user states a fact about their project, environment, or preferences that should persist
- The user says "remember," "keep in mind," "going forward," "always," or "never" followed by a constraint
- The user expresses frustration about a repeated mistake
- You realize mid-response that a prior assumption was incorrect

```bash
engram store "The correct fact here." --scope project:myproject --wrong "What was assumed incorrectly" --tags "synonym1,synonym2,related-concept,category,broader-term"
```

**Always generate rich tags** (5-10 per correction). Include synonyms, related concepts, broader categories, and the wrong thing if applicable. Tags are indexed by FTS5 and are critical for retrieval — the AI understands the semantic context at store time, so front-load that intelligence into tags rather than relying on search to infer it later.

**Retrieving corrections** — the hook handles this automatically at session start. The LLM can also call `engram get` manually when the topic shifts significantly.

```bash
engram get "current topic or task"
```

**Scope inference** — infer the correct scope automatically:
- `global`: preferences, communication style, general facts about the user
- `project:<name>`: anything specific to the current codebase (use detected .engram project name)
- `domain:<tag>`: facts about a technology or tool regardless of project (e.g. `domain:go`, `domain:sqlite`)

**Natural language commands** — the user can manage engram in plain English:
- "remember that X" → `engram store "X"`
- "forget X" → `engram list` to find it, then `engram delete <id>`
- "what do you know about this project?" → `engram list --scope project:<name>`
- "show me everything you've remembered" → `engram list`

**Acknowledgment** — after storing a correction, include a brief one-line confirmation: "▣ Stored in engram memory: <short summary>". Then continue naturally.

**Corrections are ground truth** — facts from engram take precedence over training data and prior assumptions.

### Trigger recognition table

| User says | Action |
|---|---|
| "Actually, we use X not Y" | `engram store "project uses X not Y" --wrong "Y" --tags "synonyms,related,concepts"` |
| "Stop doing that" | `engram store "user preference against that behavior" --tags "preference,style,related-terms"` |
| "I told you this before" | `engram store` + acknowledge the repeated mistake |
| "Remember: X" | `engram store "X" --tags "rich,relevant,tags"` |
| "You always get X wrong" | `engram store` with `--wrong` and `--tags` noting the pattern |
| "Forget what you said about X" | `engram list`, find it, `engram delete <id>` |
| "What have you remembered?" | `engram list` for current scope |

---

## Data model

### Schema

```sql
CREATE TABLE corrections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    fact        TEXT NOT NULL,          -- the correction in plain English
    wrong       TEXT,                  -- what was wrong (optional, analytics only, never injected)
    scope       TEXT NOT NULL,         -- 'global' | 'project:<n>' | 'domain:<tag>'
    tags        TEXT,                  -- comma-separated tags
    source      TEXT,                  -- 'user' (explicit) | 'inferred' (LLM detected)
    confidence  REAL DEFAULT 1.0,      -- 1.0 = explicit correction, 0.7 = inferred
    created_at  INTEGER NOT NULL,      -- unix timestamp
    updated_at  INTEGER NOT NULL,
    hit_count   INTEGER DEFAULT 0,     -- times injected into a session
    last_hit    INTEGER                -- unix timestamp of last injection
);

CREATE VIRTUAL TABLE corrections_fts USING fts5(
    fact, wrong, tags,
    content='corrections', content_rowid='id',
    tokenize='porter ascii'
);
```

### What a good correction looks like

Each `fact` is a single atomic English statement. Not a conversation excerpt. Not a paragraph. One fact.

Good:
```
The dev server must bind to port 8080; port 3000 is reserved for the frontend proxy.
This project uses BurntSushi/toml for config parsing, not viper.
Prefer compact prose over bullet points in all responses.
```

Bad:
```
We talked about how the project works and some things that were wrong with prior responses.
```

---

## CLI interface

```
engram init               Global init: create config and DB
engram init --project     Set up engram in current project (marker + hooks)
engram init --hooks       Reinstall just the Claude Code integration
engram store <fact>       Store a correction (with --scope, --wrong, --tags, --source flags)
engram get [query]        Retrieve relevant corrections (with --all, --raw, --limit, --scope flags)
engram list               List all corrections (with --scope, --tag, --limit flags)
engram search <query>     Search corrections with BM25 scores
engram delete <id>        Delete a correction by ID
engram edit <id>          Edit a correction in $EDITOR
engram stats              Hit counts, total corrections, estimated token savings
engram export             Export all corrections as JSON or TOML
engram import <file>      Import corrections from JSON or TOML

Global flag:
  --db <path>             Skip config loading, use database file directly (fastest)
```

## Claude Code slash commands

Installed by `engram init --hooks` into `.claude/commands/`:

| Command | What it does |
|---|---|
| `/remember` | Store a correction, preference, or fact |
| `/forget` | Find and delete a stored correction |
| `/recall` | Retrieve relevant corrections for the current topic |
| `/corrections` | List, search, export, and manage all corrections |

---

## Hook config

Run `engram init --hooks` to install automatically, or add manually to `.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "engram hook 2>/dev/null || true"
          }
        ]
      }
    ]
  }
}
```

The hook injects all corrections for the current scope at every prompt. If engram isn't installed or the DB is empty, the `|| true` ensures it fails silently.

---

## Config file

Location: `~/.config/engram/config.toml` (XDG-compliant; override with `ENGRAM_CONFIG`).

```toml
[database]
path = "~/.local/share/engram/engram.db"

[injection]
max_corrections = 10
max_tokens      = 300
min_score       = 0.0         # minimum BM25 score to include

[log]
level = "warn"
file  = ""                    # empty = stderr
```

---

## Retrieval algorithm

1. Receive query (from `engram get` or hook).
2. Walk up the directory tree for `.engram`. Extract project name if found.
3. Query `corrections_fts` filtered to applicable scopes: always `global`, plus any detected `project:` and relevant `domain:` scopes.
4. Rank by BM25 score. Apply `min_score` threshold.
5. Fill token budget: `global` first, then BM25-ranked project/domain.
6. Format as `<engram-memory>` block and output to stdout.
7. Increment `hit_count` on each used correction.

No embedding inference. BM25 over FTS5 handles typical correction sets (hundreds to low thousands) in under 5ms.

---

## Repository structure

```
engram/
├── CLAUDE.md
├── README.md
├── LICENSE
├── Makefile                   <- build/test/bench targets (handles CGO flags)
├── go.mod
├── go.sum
├── main.go                    <- cobra root command
├── .engram                    <- project scope marker
├── .claude/
│   └── commands/              <- slash commands installed by engram init --hooks
│       ├── remember.md
│       ├── forget.md
│       ├── recall.md
│       └── corrections.md
├── cmd/
│   ├── root.go                <- root command + global --db flag
│   ├── init.go                <- engram init [--project] [--hooks]
│   ├── store.go               <- engram store <fact>
│   ├── get.go                 <- engram get [query] [--all] [--raw]
│   ├── list.go
│   ├── search.go
│   ├── delete.go
│   ├── edit.go
│   ├── stats.go
│   ├── export.go
│   ├── importcmd.go
│   └── helpers.go             <- shared openDB() with --db fast path
├── internal/
│   ├── db/
│   │   ├── db.go              <- connection, migrations, WAL setup
│   │   ├── corrections.go     <- CRUD
│   │   ├── fts.go             <- FTS5 BM25 retrieval + LIKE fallback
│   │   ├── log.go             <- injection stats
│   │   └── bench_test.go      <- performance benchmarks
│   ├── project/
│   │   └── detect.go          <- walk dirs to find .engram
│   ├── config/
│   │   └── config.go          <- TOML load/validate/defaults
│   └── format/
│       └── memory_block.go    <- engram-memory block renderer
├── schema/
│   └── 001_initial.sql        <- base schema (go:embed)
└── testdata/
    └── sample_corrections.json
```

---

## Implementation notes

### Building

Use the Makefile — it handles all CGO flags automatically:

```bash
make build       # build binary
make test        # run all tests
make bench       # run benchmarks
make install     # install to /usr/local/bin
```

Or manually: `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" CGO_LDFLAGS="-lm" go build .`

### Schema migrations

Simple `schema_version` table and embedded SQL files. No ORM, no migration library.

### SQLite pragmas

Set on every connection open: WAL mode, synchronous=NORMAL, foreign_keys=ON, temp_store=MEMORY, mmap_size=128MB.

### FTS5 trigger sync

SQL triggers keep the FTS index in sync with the corrections table automatically on INSERT, UPDATE, and DELETE.

### Token estimation

`len(text) / 4` as a fast approximation. No tokenizer dependency.

---

## What NOT to do

- Do not store full conversation logs. Only atomic facts.
- Do not use vector embeddings on the retrieval path. BM25 is sufficient.
- Do not phone home. No analytics, no update checks, no cloud sync.
- Do not require a database server. SQLite only.
- Do not add a web UI, daemon, or server of any kind.
- Do not import LangChain, LlamaIndex, or any Python-ecosystem port. Pure Go.
- After storing a correction, briefly acknowledge it with "▣ Stored in engram memory: <summary>". Do not over-explain.
- Do not store the `wrong` field in injected prompts. Analytics only.
- Do not ask the user to confirm before storing a correction. Just store it.

---

## Success criteria

- `engram init` completes in under 1 second including DB creation
- `engram get` returns in under 5ms for a corpus of 10,000 corrections
- Binary size under 15MB
- Works fully offline
- A correction stored mid-session is retrievable immediately
- User onboards with two commands: `engram init` and one hook config entry
- Import/export round-trips losslessly
- Zero visible engram UI during a normal AI conversation

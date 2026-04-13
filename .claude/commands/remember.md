Store a correction, clarification, or fact in engram's persistent memory.

## Instructions

The user wants to store something in engram. They may provide the fact directly or you may need to infer it from the conversation.

### Step 1: Pre-store similarity check

Before storing, always search for near-duplicates:

```bash
engram search "<2-4 key terms from the fact>" --limit 5
```

Based on results:
- **Exact duplicate found**: Skip the store. Say "Already stored as #N."
- **Very similar found**: Update the existing one with `engram edit <id>`, or store new with `--supersedes <id>`.
- **Related but distinct**: Store new, optionally with `--supersedes <id>` if it replaces the old one.
- **No results**: Store as new.

### Step 2: Determine all fields

**fact** — one atomic sentence, normalized:
- Lead with the subject: "This project uses X" not "X is used here"
- Use fully qualified names: "BurntSushi/toml" not "toml", "filippo.io/age" not "age"
- Avoid pronouns: "The auth module requires a TTY" not "It requires a TTY"
- Include the negation when relevant: "Use X, not Y"

**type** — choose the most accurate:
- `fact` — a technical fact about the project or environment
- `preference` — a style, format, or workflow preference
- `constraint` — something that must never be violated (data safety, architecture rules)
- `gotcha` — a known trap or non-obvious behavior to avoid
- `process` — a workflow step or procedure to follow

**scope** — infer from context:
- `global` — applies to all projects and conversations
- `project:<n>` — specific to the current codebase (auto-detect from `.engram` file or project context)
- `domain:<tag>` — specific to a technology regardless of project (e.g. `domain:go`, `domain:sqlite`)

**trigger** — a short phrase describing WHEN this correction should surface. Write it from your full understanding of the current context. This is not a keyword list — it's a situation description:
- "when writing config loading or dependency management code"
- "when suggesting authentication libraries or patterns"
- "when the user asks about database connection setup"
- "when writing or reviewing test files"

This is the most valuable field after `fact`. Write it thoughtfully.

**tags** — 5-10 terms for retrieval. Include:
- Synonyms for key terms in the fact
- The wrong thing (if applicable) — so searching for the mistake finds the correction
- Related concepts someone might be thinking about
- Category/domain words

**wrong** — what was previously assumed or stated incorrectly (optional but valuable)

**supersedes** — ID of the correction this replaces (if identified in Step 1)

### Step 3: Run the store command

```bash
engram store "<fact>" \
  --type <type> \
  --scope <scope> \
  --trigger "<when to surface this>" \
  --tags "<comma,separated,tags>" \
  [--wrong "<what was wrong>"] \
  [--supersedes <id>]
```

### Step 4: Respond

Say "Got it." or "Noted." Do not narrate the engram operation. Do not say "I've stored that in engram." Do not explain what you stored.

## Examples

**User corrects a library choice:**
```bash
# Step 1
engram search "config toml viper" --limit 5

# Step 2 (assuming no duplicate found)
engram store "This project uses BurntSushi/toml for config parsing, not viper." \
  --type fact \
  --scope project:myproject \
  --trigger "when writing config loading, dependency management, or suggesting config libraries" \
  --tags "config,toml,burntsushi,viper,parsing,configuration,settings,dependencies" \
  --wrong "viper"
```

**User states a hard constraint:**
```bash
engram store "The write serialization queue in internal/queue must never be removed — it prevents a data corruption race condition." \
  --type constraint \
  --scope project:myproject \
  --trigger "when refactoring the queue package, write path, or database layer" \
  --tags "queue,write,serialization,race,data-corruption,internal/queue,concurrency"
```

**User states a preference:**
```bash
engram store "Prefer compact prose over bullet points in all responses." \
  --type preference \
  --scope global \
  --trigger "when formatting any response" \
  --tags "formatting,style,prose,bullets,lists,markdown,output,presentation"
```

**User describes a gotcha:**
```bash
engram store "Go 1.22 does not support range over integers — that was added in Go 1.23." \
  --type gotcha \
  --scope domain:go \
  --trigger "when writing range loops, discussing Go version compatibility, or suggesting language features" \
  --tags "go,golang,range,integers,1.22,1.23,version,compatibility,loop,iteration" \
  --wrong "range over integers available in Go 1.22"
```

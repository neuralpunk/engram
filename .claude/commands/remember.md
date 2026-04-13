Store a correction, clarification, or fact in engram's persistent memory.

## Instructions

The user wants to store something in engram. They may provide the fact directly as an argument, or you may need to infer it from the conversation context.

1. Determine the **fact** — a single, atomic English sentence stating what is correct.
2. Determine the **scope**:
   - `global` — preferences, communication style, general facts about the user
   - `project:<name>` — facts specific to the current codebase. Auto-detect the project name by running: `engram get --all --raw --workdir . 2>/dev/null | head -1` or check for a `.engram` file in the repo root.
   - `domain:<tag>` — facts about a technology regardless of project (e.g. `domain:go`, `domain:rust`)
3. Determine if there's a **wrong** value — what was previously assumed incorrectly.
4. **Always generate rich tags.** Think about what words someone might use to search for this fact later — synonyms, related concepts, broader categories, the technology area. This is critical for retrieval quality. Aim for 5-10 tags per correction.
5. Run the store command via Bash:

```bash
engram store "<fact>" --scope <scope> --tags "<rich,comma,separated,tags>" [--wrong "<what was wrong>"]
```

6. Respond briefly: "Got it." or "Noted." Do not narrate the engram operation.

## Tag generation

Tags are how engram finds corrections later. The AI writing the correction understands the context better than any search algorithm, so front-load that intelligence into tags. Include:
- **Synonyms** for key terms in the fact
- **Related concepts** someone might be thinking about when this fact is relevant
- **Category/domain** words (e.g. "encryption", "formatting", "dependency")
- **The wrong thing** if applicable (so searching for the mistake finds the correction)

## Examples

User says: "Remember that we use BurntSushi/toml for config"
→ `engram store "This project uses BurntSushi/toml for config parsing, not viper." --scope project:myproject --wrong "viper" --tags "config,toml,parsing,burntsushi,viper,configuration,settings"`

User says: "I prefer prose over bullet points"
→ `engram store "Prefer compact prose over bullet points in all responses." --scope global --tags "formatting,style,output,bullets,lists,markdown,writing,presentation"`

User says: "Go 1.22 doesn't support range over integers"
→ `engram store "Go 1.22 does not support range over integers, that was added in Go 1.23." --scope domain:go --tags "go,golang,version,range,iteration,loop,integers,compatibility,language-features"`

User corrects you mid-conversation:
→ `engram store "<the correct fact>" --scope <inferred> --wrong "<what you said wrong>" --tags "<rich tags>" --source inferred`

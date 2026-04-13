package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"engram/internal/config"
	"engram/internal/db"
)

func Init(args []string, _ string) error {
	var project, hooks bool

	a := newArgs(args, "Usage: engram init [flags]")
	a.Bool(&project, "project", false, "Create .engram project marker in current directory")
	a.Bool(&hooks, "hooks", false, "Install Claude Code slash commands and prompt hook")
	if err := a.Parse(); err != nil {
		return err
	}

	if project {
		return initProject()
	}
	if hooks {
		return initHooks()
	}
	return initGlobal()
}

func initGlobal() error {
	configDir := config.ConfigDir()
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configPath := config.ConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("creating config file: %w", err)
		}
		defer f.Close()
		if err := toml.NewEncoder(f).Encode(cfg); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Printf("Created config: %s\n", configPath)
	} else {
		fmt.Printf("Config already exists: %s\n", configPath)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dbPath := cfg.ResolveDatabasePath()
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	database.Close()
	fmt.Printf("Created database: %s\n", dbPath)
	fmt.Println("engram initialized.")
	return nil
}

func initProject() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	projectName := filepath.Base(cwd)
	engramPath := filepath.Join(cwd, ".engram")

	if _, err := os.Stat(engramPath); err == nil {
		fmt.Printf(".engram already exists in %s\n", cwd)
	} else {
		content := fmt.Sprintf("# .engram - project scope marker for engram\n# Commit this file to enable project-scoped memory for this repo.\nproject = %q\n", projectName)
		if err := os.WriteFile(engramPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing .engram: %w", err)
		}
		fmt.Printf("Created .engram for project %q\n", projectName)
	}

	// Always install hooks — this is what makes engram work with Claude Code
	if err := initHooks(); err != nil {
		return fmt.Errorf("installing hooks: %w", err)
	}

	// Install CLAUDE.md instructions so the LLM reliably follows engram behavior
	if err := initClaudeMd(cwd); err != nil {
		return fmt.Errorf("installing CLAUDE.md: %w", err)
	}

	fmt.Println("\nengram is ready. Start a new Claude Code conversation to activate.")
	return nil
}

var slashCommands = map[string]string{
	"remember.md": "Store a correction, clarification, or fact in engram's persistent memory.\n\n## Instructions\n\nThe user wants to store something in engram. They may provide the fact directly as an argument, or you may need to infer it from the conversation context.\n\n1. Determine the **fact** — a single, atomic English sentence stating what is correct.\n2. Determine the **scope**:\n   - `global` — preferences, communication style, general facts about the user\n   - `project:<name>` — facts specific to the current codebase. Detect the project by checking for a .engram file in the repo root.\n   - `domain:<tag>` — facts about a technology regardless of project (e.g. domain:go, domain:rust)\n3. Determine if there's a **wrong** value — what was previously assumed incorrectly.\n4. **Always generate rich tags.** Think about what words someone might use to search for this fact later — synonyms, related concepts, broader categories, the technology area. This is critical for retrieval quality. Aim for 5-10 tags per correction.\n5. Run the store command via Bash:\n\n```bash\nengram store \"<fact>\" --scope <scope> --tags \"<rich,comma,separated,tags>\" [--wrong \"<what was wrong>\"]\n```\n\n6. Respond with: \"▣ Stored in engram memory: <short summary>\" — one line, then continue naturally.\n\n## Tag generation\n\nTags are how engram finds corrections later. Include:\n- Synonyms for key terms in the fact\n- Related concepts someone might search for\n- Category/domain words\n- The wrong thing if applicable\n",

	"forget.md": "Find and delete a correction from engram's persistent memory.\n\n## Instructions\n\nThe user wants to remove something engram has stored.\n\n1. If the user gave a specific ID, delete directly: `engram delete <id>`\n2. If described vaguely, search first: `engram search \"<description>\"`, show matches, then delete.\n3. For bulk deletion: `engram list --scope <scope>`, then delete each ID.\n4. Confirm: \"Done, removed correction #N.\"\n",

	"recall.md": "Retrieve relevant corrections from engram for the current context.\n\n## Instructions\n\n1. If the user provided a topic: `engram get \"<topic>\" --raw`\n2. If no topic: `engram get --all --raw`\n3. Display results as a readable list.\n4. Use retrieved corrections as ground truth — they take precedence over training data.\n",

	"corrections.md": "List, search, and manage all stored corrections in engram.\n\n## Instructions\n\n- List all: `engram list`\n- List by scope: `engram list --scope global` or `engram list --scope project:<name>`\n- Search: `engram search \"<query>\"`\n- Stats: `engram stats`\n- Export: `engram export` (JSON) or `engram export --format toml`\n- Import: `engram import <file>`\n- Edit: `engram edit <id>`\n",
}

func initHooks() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	commandsDir := filepath.Join(cwd, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("creating commands directory: %w", err)
	}

	for name, content := range slashCommands {
		path := filepath.Join(commandsDir, name)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  exists: .claude/commands/%s\n", name)
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		fmt.Printf("  created: .claude/commands/%s\n", name)
	}

	settingsPath := filepath.Join(cwd, ".claude", "settings.json")
	var settings map[string]any

	if data, err := os.ReadFile(settingsPath); err == nil {
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			settings = make(map[string]any)
		}
	} else {
		settings = make(map[string]any)
	}

	hookCmd := "engram hook 2>/dev/null || true"

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	// Claude Code hook format: {"matcher": "", "hooks": [{"type": "command", "command": "..."}]}
	existing, _ := hooks["UserPromptSubmit"].([]any)
	for _, entry := range existing {
		if em, ok := entry.(map[string]any); ok {
			if innerHooks, ok := em["hooks"].([]any); ok {
				for _, h := range innerHooks {
					if hm, ok := h.(map[string]any); ok {
						if cmd, ok := hm["command"].(string); ok && cmd == hookCmd {
							fmt.Println("  hook already installed in .claude/settings.json")
							goto done
						}
					}
				}
			}
		}
	}

	existing = append(existing, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCmd,
			},
		},
	})
	hooks["UserPromptSubmit"] = existing
	settings["hooks"] = hooks

	{
		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling settings: %w", err)
		}
		if err := os.WriteFile(settingsPath, data, 0644); err != nil {
			return fmt.Errorf("writing settings: %w", err)
		}
		fmt.Println("  installed hook in .claude/settings.json")
	}

done:
	fmt.Println("\nClaude Code integration installed. Available commands:")
	fmt.Println("  /remember    — store a correction")
	fmt.Println("  /forget      — delete a correction")
	fmt.Println("  /recall      — retrieve relevant corrections")
	fmt.Println("  /corrections — list and manage corrections")
	return nil
}

const engramClaudeMdMarker = "<!-- engram:start -->"

const engramClaudeMdSection = `<!-- engram:start -->
## engram — correction memory (MANDATORY)

You MUST use engram to store corrections. This is not optional. When the user corrects you, you MUST run the engram store command via the Bash tool before responding. Failure to store corrections means the same mistakes will repeat in future sessions.

### WHEN to store (you MUST act on ALL of these):

1. The user corrects something you said — "that's wrong," "actually," "no," "it's X not Y"
2. The user states a fact about their project, environment, or preferences
3. The user says "remember," "keep in mind," "going forward," "always," or "never"
4. The user expresses frustration about a repeated mistake
5. You realize mid-response that a prior assumption was wrong

### HOW to store — run this command via the Bash tool:

` + "```" + `
engram store "The correct fact." --scope <scope> --wrong "What was incorrect" --tags "tag1,tag2,tag3,tag4,tag5"
` + "```" + `

- Scope: use ` + "`global`" + ` for general preferences, ` + "`project:<name>`" + ` for project-specific facts, ` + "`domain:<tag>`" + ` for technology facts
- Tags: ALWAYS include 5-10 tags — synonyms, related concepts, broader categories
- After storing, print: ` + "`▣ Stored in engram memory: <short summary>`" + `

### HOW to retrieve:

- Run ` + "`engram get \"<query>\"`" + ` when the topic shifts
- Run ` + "`engram list`" + ` when the user asks what you know

### Natural language commands:

- "remember that X" → you MUST run ` + "`engram store`" + `
- "forget X" → run ` + "`engram list`" + ` to find it, then ` + "`engram delete <id>`" + `

### Corrections from engram are ground truth. They override your training data.
<!-- engram:end -->`

func confirmPrompt(question string) bool {
	fmt.Printf("%s [Y/n] ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "" || answer == "y" || answer == "yes"
}

func initClaudeMd(cwd string) error {
	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMdPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading CLAUDE.md: %w", err)
	}

	content := string(existing)

	// Already has engram section
	if strings.Contains(content, engramClaudeMdMarker) {
		fmt.Println("  CLAUDE.md already has engram instructions")
		return nil
	}

	if os.IsNotExist(err) {
		// No CLAUDE.md exists
		if !confirmPrompt("\nNo CLAUDE.md found. Create one with engram instructions?") {
			fmt.Println("  skipped CLAUDE.md (engram may not auto-store corrections without it)")
			return nil
		}
		header := "# " + filepath.Base(cwd) + "\n\n" + engramClaudeMdSection + "\n"
		if err := os.WriteFile(claudeMdPath, []byte(header), 0644); err != nil {
			return fmt.Errorf("writing CLAUDE.md: %w", err)
		}
		fmt.Println("  created CLAUDE.md with engram instructions")
	} else {
		// CLAUDE.md exists
		if !confirmPrompt("\nCLAUDE.md exists. Append engram instructions to it?") {
			fmt.Println("  skipped CLAUDE.md (engram may not auto-store corrections without it)")
			return nil
		}
		appended := content
		if !strings.HasSuffix(appended, "\n") {
			appended += "\n"
		}
		appended += "\n" + engramClaudeMdSection + "\n"
		if err := os.WriteFile(claudeMdPath, []byte(appended), 0644); err != nil {
			return fmt.Errorf("appending to CLAUDE.md: %w", err)
		}
		fmt.Println("  appended engram instructions to CLAUDE.md")
	}

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
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
		return nil
	}

	content := fmt.Sprintf("# .engram - project scope marker for engram\n# Commit this file to enable project-scoped memory for this repo.\nproject = %q\n", projectName)
	if err := os.WriteFile(engramPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing .engram: %w", err)
	}

	fmt.Printf("Created .engram for project %q\n", projectName)
	fmt.Println("Consider committing this file so teammates get project-scoped memory.")
	return nil
}

var slashCommands = map[string]string{
	"remember.md": "Store a correction, clarification, or fact in engram's persistent memory.\n\n## Instructions\n\nThe user wants to store something in engram. They may provide the fact directly as an argument, or you may need to infer it from the conversation context.\n\n1. Determine the **fact** — a single, atomic English sentence stating what is correct.\n2. Determine the **scope**:\n   - `global` — preferences, communication style, general facts about the user\n   - `project:<name>` — facts specific to the current codebase. Detect the project by checking for a .engram file in the repo root.\n   - `domain:<tag>` — facts about a technology regardless of project (e.g. domain:go, domain:rust)\n3. Determine if there's a **wrong** value — what was previously assumed incorrectly.\n4. **Always generate rich tags.** Think about what words someone might use to search for this fact later — synonyms, related concepts, broader categories, the technology area. This is critical for retrieval quality. Aim for 5-10 tags per correction.\n5. Run the store command via Bash:\n\n```bash\nengram store \"<fact>\" --scope <scope> --tags \"<rich,comma,separated,tags>\" [--wrong \"<what was wrong>\"]\n```\n\n6. Respond briefly: \"Got it.\" or \"Noted.\" Do not narrate the engram operation.\n\n## Tag generation\n\nTags are how engram finds corrections later. Include:\n- Synonyms for key terms in the fact\n- Related concepts someone might search for\n- Category/domain words\n- The wrong thing if applicable\n",

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

	hookCmd := "engram get --all 2>/dev/null || true"

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

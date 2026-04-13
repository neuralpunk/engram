package main

import (
	"fmt"
	"os"
	"strings"

	"engram/cmd"
)

const version = "0.5.0"

const usage = `engram - correction memory for LLMs

Usage:
  engram <command> [flags]

Commands:
  init       Initialize config, database, project marker, or hooks
  store      Store a correction or clarification
  get        Retrieve relevant corrections
  list       List stored corrections
  search     Search corrections with relevance scores
  delete     Delete a correction by ID
  edit       Edit a correction in $EDITOR
  stats      Show usage statistics
  export     Export corrections as JSON or TOML
  import     Import corrections from JSON or TOML
  vacuum     Rebuild FTS index and optimize database
  hook       Claude Code hook handler (reads prompt from stdin, detects corrections)
  mcp        Start MCP stdio server (for Cursor, Windsurf, etc.)

Global flags:
  --db <path>   Use database directly (skips config loading)
  --version     Show version
  --help        Show help

Run 'engram <command> --help' for command-specific help.`

func main() {
	args := os.Args[1:]

	// Extract global flags before dispatching
	var dbPath string
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			i++
			dbPath = args[i]
		case strings.HasPrefix(args[i], "--db="):
			dbPath = args[i][len("--db="):]
		case args[i] == "--version" || args[i] == "-v":
			fmt.Printf("engram %s\n", version)
			os.Exit(0)
		default:
			filtered = append(filtered, args[i])
		}
	}
	args = filtered

	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(usage)
		os.Exit(0)
	}

	command := args[0]
	cmdArgs := args[1:]

	commands := map[string]func([]string, string) error{
		"init":   cmd.Init,
		"store":  cmd.Store,
		"get":    cmd.Get,
		"list":   cmd.List,
		"search": cmd.Search,
		"delete": cmd.Delete,
		"edit":   cmd.Edit,
		"stats":  cmd.Stats,
		"export": cmd.Export,
		"import": cmd.Import,
		"vacuum":  cmd.Vacuum,
		"hook":    cmd.Hook,
		"mcp":     cmd.MCP,
		"capture": cmd.Capture,
	}

	fn, ok := commands[command]
	if !ok {
		fmt.Fprintf(os.Stderr, "engram: unknown command %q\n\nRun 'engram --help' for usage.\n", command)
		os.Exit(1)
	}

	if err := fn(cmdArgs, dbPath); err != nil {
		if cmd.IsHelp(err) {
			fmt.Println(err)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "engram %s: %v\n", command, err)
		os.Exit(1)
	}
}

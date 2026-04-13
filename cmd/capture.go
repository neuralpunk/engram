package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// captureInput is the JSON structure Claude Code passes to PostToolUse hooks via stdin.
type captureInput struct {
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

type bashInput struct {
	Command string `json:"command"`
}

const correctionPendingFile = ".correction_pending"
const staleThresholdSecs = 300

// Capture runs as a PostToolUse hook to ensure corrections are stored.
func Capture(args []string, dbPath string) error {
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 1*1024*1024))
	if err != nil || len(input) == 0 {
		return nil
	}

	var ci captureInput
	if err := json.Unmarshal(input, &ci); err != nil {
		return nil
	}

	if ci.ToolName != "Bash" {
		return nil
	}

	stateFile := correctionStatePathFunc()
	ts, _, err := readPendingState(stateFile)
	if err != nil {
		// No state file or unreadable — nothing to do
		return nil
	}

	// Check staleness
	if time.Now().Unix()-ts > staleThresholdSecs {
		os.Remove(stateFile)
		return nil
	}

	// Check if this bash call contained an engram store/delete
	var bi bashInput
	if err := json.Unmarshal(ci.ToolInput, &bi); err == nil {
		if strings.Contains(bi.Command, "engram store") || strings.Contains(bi.Command, "engram delete") {
			os.Remove(stateFile)
			return nil
		}
	}

	// Correction pending but not stored — print reminder and delete state file
	fmt.Print(`⚠️ engram reminder: A correction was detected in the user's last message but has not been stored yet.
Store it now before continuing:
  engram store "<the correction>" --scope <scope> --tags "<tags>"
Then include "▣ Stored in engram memory: <summary>" in your response.`)

	os.Remove(stateFile)
	return nil
}

// correctionStatePathFunc is the function that returns the state file path.
// Overridable in tests.
var correctionStatePathFunc = correctionStatePath

// correctionStatePath returns the path to the correction pending state file.
func correctionStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), correctionPendingFile)
	}
	return filepath.Join(home, ".local", "share", "engram", correctionPendingFile)
}

// writePendingState writes the correction pending state file atomically.
func writePendingState(prompt string) {
	stateFile := correctionStatePathFunc()
	dir := filepath.Dir(stateFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return
	}

	// Truncate prompt to 120 runes, single line
	runes := []rune(prompt)
	if len(runes) > 120 {
		runes = runes[:120]
	}
	snippet := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, string(runes))

	content := fmt.Sprintf("%d\n%s\n", time.Now().Unix(), snippet)

	// Write atomically: temp file + rename
	tmp := stateFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return
	}
	os.Rename(tmp, stateFile)
}

// readPendingState reads the state file. Returns timestamp and snippet, or error if missing/invalid.
func readPendingState(path string) (int64, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	lines := strings.SplitN(string(data), "\n", 3)
	if len(lines) < 2 {
		return 0, "", fmt.Errorf("invalid state file")
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid timestamp: %w", err)
	}
	return ts, strings.TrimSpace(lines[1]), nil
}

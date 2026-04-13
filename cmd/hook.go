package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"engram/internal/db"
	"engram/internal/format"
	"engram/internal/project"
)

// hookInput is the JSON structure Claude Code passes to UserPromptSubmit hooks via stdin.
type hookInput struct {
	Prompt string `json:"prompt"`
}

// correctionPatterns matches common correction phrases in user messages.
var correctionPatterns = regexp.MustCompile(
	`(?i)` +
		`\bactually[\s,]` +
		`|\bthat'?s (wrong|not right|not correct|incorrect|not true)` +
		`|\bno[,.]?\s+(it|that|we|I|you|the)\b` +
		`|\bnot (right|correct|true)\b` +
		`|\byou'?re (wrong|mistaken|incorrect)` +
		`|\bI told you\b` +
		`|\bstop doing\b` +
		`|\bwe (don'?t|do not) use\b` +
		`|\bwe use .+ not\b` +
		`|\bit'?s .+ not .+` +
		`|\bwrong[.!]` +
		`|\bI('?ve| have) (already|told|said|mentioned)\b` +
		`|\bplease (stop|don'?t|do not)\b` +
		`|\bfor the last time\b` +
		`|\bhow many times\b` +
		`|\bremember (that|this|:)\b` +
		`|\bkeep in mind\b` +
		`|\bgoing forward\b` +
		`|\balways .+ never\b` +
		`|\bnever .+ always\b`,
)

func Hook(args []string, dbPath string) error {
	// Read hook input from stdin
	var prompt string
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 10*1024*1024)) // 10MB limit
	if err == nil && len(input) > 0 {
		var hi hookInput
		if json.Unmarshal(input, &hi) == nil {
			prompt = hi.Prompt
		}
	}

	// Open database and get corrections (same as engram get --all)
	database, cfg, err := openDB(dbPath)
	if err != nil {
		// If DB fails, still output behavior prompt with no corrections
		fmt.Print(format.FormatSystemPrompt(nil))
		if prompt != "" && correctionPatterns.MatchString(prompt) {
			printCorrectionAlert()
		}
		return nil
	}

	detectedProject := ""
	if projName, found := project.Detect("."); found {
		detectedProject = projName
	}

	var scopes []string
	if detectedProject != "" {
		scopes = []string{"global", "project:" + detectedProject}
	}

	corrections, err := database.List("", "", 0)
	if err != nil {
		database.Close()
		fmt.Print(format.FormatSystemPrompt(nil))
		if prompt != "" && correctionPatterns.MatchString(prompt) {
			printCorrectionAlert()
		}
		return nil
	}

	if len(scopes) > 0 {
		scopeSet := make(map[string]bool, len(scopes))
		for s := range scopes {
			scopeSet[scopes[s]] = true
		}
		filtered := corrections[:0:0]
		for _, c := range corrections {
			if scopeSet[c.Scope] {
				filtered = append(filtered, c)
			}
		}
		corrections = filtered
	}

	scored := make([]db.ScoredCorrection, len(corrections))
	for i, c := range corrections {
		scored[i] = db.ScoredCorrection{Correction: c, Score: -1.0}
	}
	selected := format.SelectCorrections(scored, cfg.Injection.MaxCorrections, cfg.Injection.MaxTokens, detectedProject)

	// Output behavior prompt + corrections
	fmt.Print(format.FormatSystemPrompt(selected))

	// If correction detected, append urgent instruction
	if prompt != "" && correctionPatterns.MatchString(prompt) {
		printCorrectionAlert()
	}

	// Update hit counts before closing
	if len(selected) > 0 {
		ids := make([]int64, len(selected))
		for i, c := range selected {
			ids[i] = c.ID
		}
		database.IncrementHitCounts(ids)
	}
	database.Close()

	return nil
}

func printCorrectionAlert() {
	alert := strings.Join([]string{
		"",
		"",
		"⚠️ CORRECTION DETECTED in the user's message.",
		"You MUST run engram store via Bash BEFORE responding to the user.",
		"Extract the correct fact from their message and store it:",
		"  engram store \"<the correct fact>\" --scope <scope> --wrong \"<what was incorrect>\" --tags \"tag1,tag2,tag3,tag4,tag5\"",
		"Then include \"▣ Stored in engram memory: <summary>\" in your response.",
		"DO NOT SKIP THIS STEP.",
	}, "\n")
	fmt.Print(alert)
}

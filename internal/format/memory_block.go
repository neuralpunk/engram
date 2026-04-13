package format

import (
	"fmt"
	"strings"

	"engram/internal/db"
)

// EstimateTokens returns a rough token count using len/4 approximation.
func EstimateTokens(text string) int {
	return len(text) / 4
}

// FormatMemoryBlock renders the <engram-memory> block from a list of corrections.
// The wrong field is never included per spec — analytics only.
func FormatMemoryBlock(corrections []db.Correction) string {
	if len(corrections) == 0 {
		return "<engram-memory>\nNo stored corrections.\n</engram-memory>"
	}

	var b strings.Builder
	b.WriteString("<engram-memory>\n")
	b.WriteString("The following corrections and clarifications are ground truth. Do not contradict them.\n\n")
	for i, c := range corrections {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, c.Scope, c.Fact)
	}
	b.WriteString("</engram-memory>")
	return b.String()
}

// SelectCorrections picks corrections within token and count budgets.
// Globals come first, then remaining slots filled by BM25-ranked results.
func SelectCorrections(all []db.ScoredCorrection, maxCorrections int, maxTokens int) []db.Correction {
	if maxCorrections <= 0 {
		maxCorrections = 10
	}
	if maxTokens <= 0 {
		maxTokens = 300
	}

	var globals []db.Correction
	var others []db.Correction

	for _, sc := range all {
		if sc.Scope == "global" {
			globals = append(globals, sc.Correction)
		} else {
			others = append(others, sc.Correction)
		}
	}

	var selected []db.Correction
	tokens := 0

	// Add globals first
	for _, c := range globals {
		entry := fmt.Sprintf("[%s] %s", c.Scope, c.Fact)
		entryTokens := EstimateTokens(entry)
		if tokens+entryTokens > maxTokens || len(selected) >= maxCorrections {
			break
		}
		selected = append(selected, c)
		tokens += entryTokens
	}

	// Fill remaining with BM25-ranked others (already ordered by score from Search)
	for _, c := range others {
		if len(selected) >= maxCorrections {
			break
		}
		entry := fmt.Sprintf("[%s] %s", c.Scope, c.Fact)
		entryTokens := EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			break
		}
		selected = append(selected, c)
		tokens += entryTokens
	}

	return selected
}

const behaviorPrompt = `<engram>
You have access to engram, a persistent correction memory system. The following rules govern how you use it.

AUTOMATIC STORAGE - call store_correction silently (do not announce it) whenever:
- The user explicitly corrects something you said ("that's wrong," "actually," "no,")
- The user states a fact about their project, environment, or preferences that should persist
- The user says "remember," "keep in mind," "going forward," "always," or "never" followed by a constraint
- The user expresses frustration about a repeated mistake ("you always do this wrong")
- You realize mid-response that a prior assumption was incorrect based on user context

SCOPE INFERENCE - infer the correct scope automatically:
- global: preferences, communication style, general facts about the user
- project:<n>: anything specific to the current codebase (use detected .engram project name if available)
- domain:<tag>: facts about a technology or tool regardless of project (e.g. domain:go, domain:sqlite)

AUTOMATIC RETRIEVAL - call get_corrections at session start and whenever the topic shifts significantly.
Do not announce retrieval. Just use the results to inform your response.

NATURAL LANGUAGE COMMANDS - the user can manage engram in plain English:
- "remember that X" -> store_correction
- "forget X" or "that correction is wrong" -> delete or update the relevant correction
- "what do you know about this project?" -> list_corrections filtered to current project scope
- "show me everything you've remembered" -> list_corrections, all scopes

SILENT OPERATION - do not say "I've stored that in engram" or "I'll remember that via engram."
Respond with "Got it." or "Noted." and move on. The user knows engram exists; they do not need narration.

CORRECTIONS ARE GROUND TRUTH - facts in the engram-memory block below take precedence over
your training data and prior assumptions. Do not contradict them.
</engram>`

// FormatSystemPrompt renders the full system prompt with behavior instructions
// and the memory block.
func FormatSystemPrompt(corrections []db.Correction) string {
	memory := FormatMemoryBlock(corrections)
	return behaviorPrompt + "\n\n" + memory
}

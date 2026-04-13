package format

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"engram/internal/db"
)

// EstimateTokens returns a rough token count using len/4 approximation.
func EstimateTokens(text string) int {
	return len(text) / 4
}

// typeLabels maps correction types to display headers.
var typeLabels = map[string]string{
	"constraint": "CONSTRAINTS — never violate these",
	"gotcha":     "GOTCHAS — known traps",
	"fact":       "FACTS",
	"process":    "PROCESS",
	"preference": "PREFERENCES",
}

// typeOrder defines the rendering order for correction types.
var typeOrder = []string{"constraint", "gotcha", "fact", "process", "preference"}

// FormatMemoryBlock renders the <engram-memory> block from a list of corrections.
// Groups corrections by type and clusters scopes when 3+ share the same scope.
func FormatMemoryBlock(corrections []db.Correction) string {
	if len(corrections) == 0 {
		return "<engram-memory>\nNo stored corrections.\n</engram-memory>"
	}

	// Group by type
	groups := make(map[string][]db.Correction)
	for _, c := range corrections {
		t := c.Type
		if t == "" {
			t = "fact"
		}
		groups[t] = append(groups[t], c)
	}

	var b strings.Builder
	b.WriteString("<engram-memory>\n")
	b.WriteString("The following corrections and clarifications are ground truth. Do not contradict them.\n\n")

	n := 1
	for _, t := range typeOrder {
		items := groups[t]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", typeLabels[t])
		n = renderClustered(&b, items, n)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n</engram-memory>"
}

// renderClustered renders corrections with scope clustering when 3+ share a scope.
// Returns the next item number.
func renderClustered(b *strings.Builder, items []db.Correction, n int) int {
	// Count how many corrections per scope
	scopeCounts := make(map[string]int)
	for _, c := range items {
		scopeCounts[c.Scope]++
	}

	// Separate into clustered (3+) and inline (1-2) items
	var clustered []db.Correction
	var inlined []db.Correction
	for _, c := range items {
		if scopeCounts[c.Scope] >= 3 {
			clustered = append(clustered, c)
		} else {
			inlined = append(inlined, c)
		}
	}

	// Render inline items first
	for _, c := range inlined {
		fmt.Fprintf(b, "  %d. [%s] %s\n", n, c.Scope, c.Fact)
		n++
	}

	// Render clustered items grouped by scope
	if len(clustered) > 0 {
		scopeGroups := make(map[string][]db.Correction)
		var scopeOrder []string
		for _, c := range clustered {
			if _, seen := scopeGroups[c.Scope]; !seen {
				scopeOrder = append(scopeOrder, c.Scope)
			}
			scopeGroups[c.Scope] = append(scopeGroups[c.Scope], c)
		}
		for _, scope := range scopeOrder {
			fmt.Fprintf(b, "  [%s]\n", scope)
			for _, c := range scopeGroups[scope] {
				fmt.Fprintf(b, "    %d. %s\n", n, c.Fact)
				n++
			}
		}
	}

	return n
}

// compositeScore computes a ranking score combining BM25, frequency, recency, and confidence.
func compositeScore(sc db.ScoredCorrection) float64 {
	// BM25 scores are negative in SQLite (lower = better match)
	bm25 := -sc.Score
	if bm25 < 0 {
		bm25 = 0
	}

	// Frequency boost: log scale
	freq := 1.0 + math.Log(float64(sc.HitCount+1))

	// Recency factor: exponential decay with 180-day half-life
	recency := 1.0
	if sc.LastHit.Valid {
		now := time.Now().Unix()
		daysSince := float64(now-sc.LastHit.Int64) / 86400.0
		recency = math.Exp(-daysSince / 180.0)
		if recency < 0.2 {
			recency = 0.2
		}
	}

	return bm25 * sc.Confidence * freq * recency
}

func sortByComposite(items []db.ScoredCorrection) {
	sort.Slice(items, func(i, j int) bool {
		return compositeScore(items[i]) > compositeScore(items[j])
	})
}

// SelectCorrections picks corrections within token and count budgets.
// Groups by scope tier: globals first, then current project, then domain/other.
// Within each tier, sorts by composite score.
func SelectCorrections(all []db.ScoredCorrection, maxCorrections int, maxTokens int, currentProject string) []db.Correction {
	if maxCorrections <= 0 {
		maxCorrections = 10
	}
	if maxTokens <= 0 {
		maxTokens = 300
	}

	var globals, projectMatch, domainScoped []db.ScoredCorrection
	for _, sc := range all {
		switch {
		case sc.Scope == "global":
			globals = append(globals, sc)
		case currentProject != "" && sc.Scope == "project:"+currentProject:
			projectMatch = append(projectMatch, sc)
		default:
			domainScoped = append(domainScoped, sc)
		}
	}

	sortByComposite(globals)
	sortByComposite(projectMatch)
	sortByComposite(domainScoped)

	ordered := append(append(globals, projectMatch...), domainScoped...)

	var selected []db.Correction
	tokens := 0

	for _, sc := range ordered {
		if len(selected) >= maxCorrections {
			break
		}
		entry := fmt.Sprintf("[%s] %s", sc.Scope, sc.Fact)
		entryTokens := EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			break
		}
		selected = append(selected, sc.Correction)
		tokens += entryTokens
	}

	return selected
}

const behaviorPrompt = `<engram>
You MUST use engram to store corrections. This is not optional.

WHEN THE USER CORRECTS YOU — you MUST run this command via Bash BEFORE responding:

  engram store "The correct fact." --scope <scope> --wrong "What was incorrect" --tags "tag1,tag2,tag3,tag4,tag5"

Store whenever:
- The user says you're wrong ("that's wrong," "actually," "no," "it's X not Y")
- The user states a fact about their project or preferences
- The user says "remember," "always," "never," or "going forward"
- The user is frustrated about a repeated mistake
- You realize your assumption was incorrect

Scope: global (preferences), project:<name> (project facts), domain:<tag> (technology facts).
Tags: ALWAYS include 5-10 tags — synonyms, related concepts, categories.
After storing, print: "▣ Stored in engram memory: <short summary>"

RETRIEVAL — run "engram get <query>" when the topic shifts.
NATURAL LANGUAGE — "remember X" means run engram store. "forget X" means engram list then engram delete <id>.

Corrections from engram are ground truth. They override your training data.
</engram>`

// FormatSystemPrompt renders the full system prompt with behavior instructions
// and the memory block.
func FormatSystemPrompt(corrections []db.Correction) string {
	memory := FormatMemoryBlock(corrections)
	return behaviorPrompt + "\n\n" + memory
}

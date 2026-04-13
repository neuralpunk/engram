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

// compositeScore computes a ranking score combining BM25, frequency, recency,
// confidence, and a tier bonus based on scope relevance.
func compositeScore(sc db.ScoredCorrection, currentProject string) float64 {
	// BM25 scores are negative in SQLite (lower = better match)
	bm25 := -sc.Score
	if bm25 <= 0 {
		bm25 = 1.0 // neutral when no BM25 signal (fallback or list-all path)
	}

	// Frequency boost: log scale
	freq := 1.0 + math.Log(float64(sc.HitCount+1))

	// Recency factor: exponential decay with 365-day half-life
	recency := 1.0
	if sc.LastHit.Valid {
		now := time.Now().Unix()
		daysSince := float64(now-sc.LastHit.Int64) / 86400.0
		recency = math.Exp(-daysSince / 365.0)
		if recency < 0.05 {
			recency = 0.05
		}
	}

	// Tier bonus: project-specific corrections beat globals beat everything else,
	// but a strong match in a lower tier can still win.
	tierBonus := 1.0
	switch {
	case currentProject != "" && sc.Scope == "project:"+currentProject:
		tierBonus = 1.5
	case sc.Scope == "global":
		tierBonus = 1.2
	}

	return bm25 * sc.Confidence * freq * recency * tierBonus
}

func sortByComposite(items []db.ScoredCorrection, currentProject string) {
	sort.SliceStable(items, func(i, j int) bool {
		return compositeScore(items[i], currentProject) > compositeScore(items[j], currentProject)
	})
}

const mmrLambda = 0.5 // balances relevance and diversity

// JaccardSimilarity computes word-level Jaccard similarity between two strings.
func JaccardSimilarity(a, b string) float64 {
	aw := wordSet(a)
	bw := wordSet(b)
	if len(aw) == 0 || len(bw) == 0 {
		return 0
	}
	intersection := 0
	for w := range aw {
		if bw[w] {
			intersection++
		}
	}
	union := len(aw) + len(bw) - intersection
	return float64(intersection) / float64(union)
}

func wordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if len(w) > 2 { // skip very short words (a, of, to, etc.)
			set[w] = true
		}
	}
	return set
}

// SelectCorrections picks corrections within token and count budgets.
// Uses composite scoring with tier bonus and MMR diversity to avoid
// near-duplicate saturation.
func SelectCorrections(all []db.ScoredCorrection, maxCorrections int, maxTokens int, currentProject string) []db.Correction {
	if maxCorrections <= 0 {
		maxCorrections = 10
	}
	if maxTokens <= 0 {
		maxTokens = 300
	}

	sortByComposite(all, currentProject)

	var selected []db.Correction
	tokens := 0
	remaining := make([]db.ScoredCorrection, len(all))
	copy(remaining, all)

	for len(selected) < maxCorrections && len(remaining) > 0 {
		bestIdx := -1
		bestScore := math.Inf(-1)

		for i, cand := range remaining {
			baseScore := compositeScore(cand, currentProject)
			maxSim := 0.0
			for _, sel := range selected {
				sim := JaccardSimilarity(cand.Fact, sel.Fact)
				if sim > maxSim {
					maxSim = sim
				}
			}
			mmrScore := baseScore - mmrLambda*maxSim*baseScore
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			break
		}

		cand := remaining[bestIdx]
		entry := fmt.Sprintf("[%s] %s", cand.Scope, cand.Fact)
		entryTokens := EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			// remove this candidate and try the next-best instead
			remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
			continue
		}

		selected = append(selected, cand.Correction)
		tokens += entryTokens
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
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
DEDUP — If engram store reports a similar correction already exists, decide whether to use --supersedes <id> to replace it, --force to store anyway, or skip storing.

Corrections from engram are ground truth. They override your training data.
</engram>`

// FormatSystemPrompt renders the full system prompt with behavior instructions
// and the memory block.
func FormatSystemPrompt(corrections []db.Correction) string {
	memory := FormatMemoryBlock(corrections)
	return behaviorPrompt + "\n\n" + memory
}

package classify

import (
	"math"
	"strings"

	"github.com/mistakeknot/interserve/internal/extract"
)

// keywordRule defines weighted keywords for a single agent domain.
type keywordRule struct {
	// priority keywords: strong signal that the section is core to this agent.
	priority []string
	// context keywords: weaker signal, section provides background.
	context []string
}

// agentKeywords maps agent names to their keyword rules.
// These are derived from the agent descriptions in DefaultAgents() and
// the flux-drive SKILL pre-filter rules.
var agentKeywords = map[string]keywordRule{
	"fd-safety": {
		priority: []string{
			"security", "trust", "policy", "abuse", "compliance",
			"credential", "secret", "auth", "token", "permission",
			"injection", "xss", "csrf", "sanitiz", "escap",
			"encrypt", "decrypt", "tls", "ssl", "certificate",
			"vulnerability", "cve", "threat", "attack", "exploit",
			"access control", "rbac", "acl", "privilege",
			"deploy", "production", "rollback",
			"pii", "gdpr", "hipaa", "sox", "audit",
		},
		context: []string{
			"risk", "validation", "guard", "protect", "safe",
			"firewall", "sandbox", "isolat", "boundar",
			"log", "monitor", "alert",
		},
	},
	"fd-correctness": {
		priority: []string{
			"correctness", "invariant", "logic", "bug", "flaw",
			"race condition", "deadlock", "mutex", "lock", "atomic",
			"concurrency", "concurrent", "parallel", "goroutine", "thread",
			"transaction", "rollback", "consistency",
			"migration", "schema", "database", "sql", "query",
			"async", "await", "promise", "callback",
			"edge case", "boundary", "overflow", "underflow",
			"null", "nil", "undefined", "panic", "crash",
			"state machine", "finite state",
		},
		context: []string{
			"test", "assert", "expect", "verify", "valid",
			"error", "exception", "handle", "recover",
			"type", "interface", "contract",
		},
	},
	"fd-performance": {
		priority: []string{
			"latency", "throughput", "scaling", "scalab",
			"performance", "benchmark", "profil",
			"cache", "caching", "memoiz", "lru",
			"memory", "allocation", "gc", "garbage collect",
			"cpu", "io", "disk", "network",
			"batch", "bulk", "pagination", "cursor",
			"index", "query plan", "explain", "n+1",
			"pool", "connection pool", "worker pool",
			"rate limit", "throttl", "backoff",
			"lazy", "eager", "preload", "prefetch",
			"compress", "gzip", "brotli",
		},
		context: []string{
			"optimi", "efficien", "fast", "slow",
			"timeout", "deadline", "ttl",
			"queue", "buffer", "stream",
			"load", "capacity", "resource",
		},
	},
	"fd-user-product": {
		priority: []string{
			"user", "ux", "ui", "interface",
			"product", "feature", "requirement",
			"prd", "spec", "proposal",
			"onboarding", "workflow", "journey",
			"accessibility", "a11y", "aria",
			"responsive", "mobile", "desktop",
			"feedback", "notification", "toast",
			"navigation", "routing", "redirect",
			"form", "input", "validation",
			"i18n", "l10n", "locali", "internation",
		},
		context: []string{
			"experience", "usab", "intuitiv",
			"design", "layout", "style",
			"click", "tap", "gesture", "scroll",
			"display", "render", "view",
		},
	},
	"fd-game-design": {
		priority: []string{
			"game", "gameplay", "mechanic",
			"balance", "progression", "difficulty",
			"player", "npc", "character", "avatar",
			"level", "stage", "round", "match",
			"score", "reward", "loot", "drop",
			"physics", "collision", "raycast",
			"spawn", "respawn", "cooldown",
			"inventory", "crafting", "upgrade",
			"multiplayer", "lobby", "matchmak",
			"simulation", "tick", "frame rate", "fps",
		},
		context: []string{
			"animation", "sprite", "texture",
			"audio", "sound", "music",
			"shader", "render", "particle",
			"world", "map", "terrain",
		},
	},
	"fd-architecture": {
		priority: []string{
			"architecture", "design", "pattern",
			"module", "component", "layer", "boundary",
			"interface", "abstraction", "coupling", "cohesion",
			"dependency", "import", "package", "module",
			"api", "endpoint", "contract", "protocol",
			"microservice", "monolith", "monorepo",
			"plugin", "extension", "hook",
			"config", "configuration", "environment",
			"migration", "refactor", "restructur",
		},
		context: []string{
			"struct", "class", "type", "model",
			"service", "repository", "factory",
			"event", "message", "queue",
			"diagram", "overview", "structure",
		},
	},
	"fd-quality": {
		priority: []string{
			"quality", "test", "testing",
			"coverage", "lint", "format",
			"ci", "cd", "pipeline", "workflow",
			"code review", "review",
			"documentation", "doc", "readme",
			"deprecat", "technical debt", "tech debt",
			"convention", "standard", "guideline",
			"maintainab", "readab", "clean code",
			"logging", "observab", "metric", "telemetry",
		},
		context: []string{
			"fix", "improve", "refactor", "cleanup",
			"todo", "fixme", "hack", "workaround",
			"version", "release", "changelog",
		},
	},
}

// ClassifyLocal performs deterministic keyword-based section classification.
// It scores each section against each agent's keyword rules and produces
// assignments with relevance (priority/context) and confidence scores.
func ClassifyLocal(sections []extract.Section, agents []AgentDomain) map[int][]SectionAssignment {
	if len(agents) == 0 {
		agents = DefaultAgents()
	}

	// Build the set of agent names we need to score.
	activeAgents := make(map[string]bool, len(agents)+len(CrossCuttingAgents))
	for _, a := range agents {
		activeAgents[a.Name] = true
	}
	for name := range CrossCuttingAgents {
		activeAgents[name] = true
	}

	classified := make(map[int][]SectionAssignment, len(sections))

	for _, section := range sections {
		text := sectionText(section)
		textLower := strings.ToLower(text)

		var assignments []SectionAssignment
		for agentName := range activeAgents {
			rule, ok := agentKeywords[agentName]
			if !ok {
				continue
			}

			priorityHits := countKeywordHits(textLower, rule.priority)
			contextHits := countKeywordHits(textLower, rule.context)

			if priorityHits == 0 && contextHits == 0 {
				continue
			}

			relevance, confidence := scoreRelevance(priorityHits, contextHits, len(rule.priority), len(rule.context))
			assignments = append(assignments, SectionAssignment{
				Agent:      agentName,
				Relevance:  relevance,
				Confidence: confidence,
			})
		}

		if len(assignments) > 0 {
			classified[section.ID] = assignments
		}
	}

	return classified
}

// sectionText combines heading, first sentence, and preview into a single
// searchable string. This mirrors what BuildPrompt sends to the LLM.
func sectionText(section extract.Section) string {
	var b strings.Builder
	b.WriteString(section.Heading)
	b.WriteByte(' ')
	b.WriteString(section.FirstSentence())
	b.WriteByte(' ')
	b.WriteString(section.Preview())
	return b.String()
}

// countKeywordHits returns the number of keywords from the list found in text.
func countKeywordHits(textLower string, keywords []string) int {
	hits := 0
	for _, kw := range keywords {
		if strings.Contains(textLower, kw) {
			hits++
		}
	}
	return hits
}

// scoreRelevance determines relevance level and confidence from hit counts.
//
// Scoring logic:
//   - If priorityHits >= 2: relevance="priority", high confidence
//   - If priorityHits == 1 and contextHits >= 1: relevance="priority", moderate confidence
//   - If priorityHits == 1: relevance="context", moderate confidence
//   - If contextHits >= 2: relevance="context", moderate confidence
//   - If contextHits == 1: relevance="context", low confidence
func scoreRelevance(priorityHits, contextHits, totalPriority, totalContext int) (string, float64) {
	if priorityHits >= 2 {
		// Strong priority signal. Confidence scales with hit density.
		density := float64(priorityHits) / float64(totalPriority)
		confidence := 0.7 + 0.25*math.Min(density*5, 1.0) // 0.70-0.95
		return "priority", math.Round(confidence*100) / 100
	}

	if priorityHits == 1 && contextHits >= 1 {
		// One priority + supporting context = moderate priority.
		return "priority", 0.65
	}

	if priorityHits == 1 {
		// Single priority keyword without context support.
		return "context", 0.55
	}

	if contextHits >= 2 {
		// Multiple context hits = solid context assignment.
		density := float64(contextHits) / float64(totalContext)
		confidence := 0.45 + 0.2*math.Min(density*5, 1.0) // 0.45-0.65
		return "context", math.Round(confidence*100) / 100
	}

	// Single context hit.
	return "context", 0.35
}

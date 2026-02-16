package classify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mistakeknot/interserve/internal/extract"
)

// AgentDomain defines a domain specialist that can receive section assignments.
type AgentDomain struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DefaultAgents returns the baseline flux-drive domain agents.
func DefaultAgents() []AgentDomain {
	return []AgentDomain{
		{Name: "fd-safety", Description: "Safety, trust, policy risk, abuse, and compliance impact."},
		{Name: "fd-correctness", Description: "Functional correctness, invariants, and logic flaws."},
		{Name: "fd-performance", Description: "Latency, throughput, scaling, and resource efficiency."},
		{Name: "fd-user-product", Description: "User value, product behavior, and UX outcome quality."},
		{Name: "fd-game-design", Description: "Systems balance, mechanics, progression, and play quality."},
	}
}

var CrossCuttingAgents = map[string]bool{
	"fd-architecture": true,
	"fd-quality":      true,
}

// BuildPrompt builds a classification prompt for Codex spark dispatch.
func BuildPrompt(sections []extract.Section, agents []AgentDomain) string {
	if len(agents) == 0 {
		agents = DefaultAgents()
	}

	var b strings.Builder
	b.WriteString("You classify markdown document sections for flux-drive review routing.\n")
	b.WriteString("Assign each section to zero or more agents with:\n")
	b.WriteString("- relevance: priority | context\n")
	b.WriteString("- confidence: 0.0 to 1.0\n")
	b.WriteString("Only use the listed agent names.\n\n")

	b.WriteString("Agent domains:\n")
	for _, agent := range agents {
		fmt.Fprintf(&b, "- %s: %s\n", agent.Name, agent.Description)
	}

	keys := make([]string, 0, len(CrossCuttingAgents))
	for name := range CrossCuttingAgents {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	b.WriteString("\nCross-cutting agents (optional):\n")
	for _, name := range keys {
		fmt.Fprintf(&b, "- %s\n", name)
	}

	b.WriteString("\nSections:\n")
	for _, section := range sections {
		heading := strings.TrimSpace(section.Heading)
		if heading == "" {
			heading = "(untitled)"
		}
		firstSentence := section.FirstSentence()
		if firstSentence == "" {
			firstSentence = "(none)"
		}
		preview := section.Preview()
		if preview == "" {
			preview = "(empty section body)"
		}

		fmt.Fprintf(&b, "\nSection %d\n", section.ID)
		fmt.Fprintf(&b, "Heading: %s\n", heading)
		fmt.Fprintf(&b, "LineCount: %d\n", section.LineCount)
		fmt.Fprintf(&b, "FirstSentence: %s\n", firstSentence)
		b.WriteString("Preview:\n")
		b.WriteString(preview)
		b.WriteString("\n")
	}

	b.WriteString("\nReturn JSON only (no markdown fences) with this schema:\n")
	b.WriteString("{\n")
	b.WriteString("  \"sections\": [\n")
	b.WriteString("    {\n")
	b.WriteString("      \"section_id\": 1,\n")
	b.WriteString("      \"assignments\": [\n")
	b.WriteString("        {\"agent\": \"fd-safety\", \"relevance\": \"priority\", \"confidence\": 0.95}\n")
	b.WriteString("      ]\n")
	b.WriteString("    }\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n")

	return b.String()
}

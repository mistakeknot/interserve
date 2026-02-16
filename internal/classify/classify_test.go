package classify

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mistakeknot/clodex/internal/extract"
)

func TestBuildPromptIncludesAgentsAndHeadings(t *testing.T) {
	agents := DefaultAgents()
	sections := []extract.Section{
		{ID: 1, Heading: "Intro", Body: "Overview", LineCount: 1},
		{ID: 2, Heading: "Safety", Body: "Guardrails", LineCount: 1},
	}

	prompt := BuildPrompt(sections, agents)

	if !strings.Contains(prompt, "fd-safety") || !strings.Contains(prompt, "Safety, trust") {
		t.Fatalf("prompt missing expected agent description")
	}
	if !strings.Contains(prompt, "Heading: Intro") || !strings.Contains(prompt, "Heading: Safety") {
		t.Fatalf("prompt missing expected section headings")
	}
}

func TestBuildPromptApproxTokenBudgetForTwentySections(t *testing.T) {
	agents := DefaultAgents()
	sections := make([]extract.Section, 0, 20)
	for i := 0; i < 20; i++ {
		sections = append(sections, extract.Section{
			ID:        i + 1,
			Heading:   fmt.Sprintf("Section-%02d", i+1),
			Body:      makeBody(120),
			LineCount: 120,
		})
	}

	prompt := BuildPrompt(sections, agents)
	approxTokens := len(prompt) / 4
	if approxTokens > 8000 {
		t.Fatalf("prompt too large: ~%d tokens (>8000)", approxTokens)
	}
}

func TestBuildResultAppliesEightyPercentThreshold(t *testing.T) {
	agents := DefaultAgents()
	t.Run("80 percent threshold upgrades agent to full doc", func(t *testing.T) {
		sections := []extract.Section{
			{ID: 1, Heading: "A", LineCount: 80},
			{ID: 2, Heading: "B", LineCount: 20},
		}
		classified := map[int][]SectionAssignment{
			1: {{Agent: "fd-safety", Relevance: "priority", Confidence: 0.9}},
			2: {{Agent: "fd-safety", Relevance: "context", Confidence: 0.7}},
		}

		result := buildResult(classified, sections, agents)
		if result.Status != "success" {
			t.Fatalf("expected success, got %q: %s", result.Status, result.Error)
		}
		slice := result.SlicingMap["fd-safety"]
		// 80% threshold → agent gets all sections as priority
		if len(slice.PrioritySections) != 2 {
			t.Fatalf("80%% threshold: expected 2 priority sections, got %d", len(slice.PrioritySections))
		}
		if slice.TotalPriorityLines != 100 {
			t.Fatalf("expected total priority lines = 100, got %d", slice.TotalPriorityLines)
		}
	})

	t.Run("below 80 percent keeps slicing", func(t *testing.T) {
		sections := []extract.Section{
			{ID: 1, Heading: "A", LineCount: 79},
			{ID: 2, Heading: "B", LineCount: 21},
		}
		classified := map[int][]SectionAssignment{
			1: {{Agent: "fd-safety", Relevance: "priority", Confidence: 0.9}},
			2: {{Agent: "fd-safety", Relevance: "context", Confidence: 0.7}},
		}

		result := buildResult(classified, sections, agents)
		if result.Status != "success" {
			t.Fatalf("expected success (79%% > 10%% mismatch guard), got %q: %s", result.Status, result.Error)
		}
		slice := result.SlicingMap["fd-safety"]
		// Below 80% → agent keeps sliced sections
		if len(slice.PrioritySections) != 1 {
			t.Fatalf("below 80%%: expected 1 priority section, got %d", len(slice.PrioritySections))
		}
	})
}

func TestBuildResultDomainMismatchGuard(t *testing.T) {
	agents := DefaultAgents()
	sections := make([]extract.Section, 0, 10)
	for i := 0; i < 10; i++ {
		sections = append(sections, extract.Section{ID: i + 1, Heading: fmt.Sprintf("S%d", i+1), LineCount: 5})
	}

	classified := map[int][]SectionAssignment{
		1: {{Agent: "fd-safety", Relevance: "priority", Confidence: 0.6}}, // 5/50 = 10%
	}

	result := buildResult(classified, sections, agents)
	if result.Status != "no_classification" {
		t.Fatalf("expected domain mismatch guard to keep no_classification, got %q", result.Status)
	}
}

func makeBody(lines int) string {
	out := make([]string, lines)
	for i := 0; i < lines; i++ {
		out[i] = fmt.Sprintf("line-%d", i+1)
	}
	return strings.Join(out, "\n")
}

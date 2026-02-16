package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/mistakeknot/clodex/internal/extract"
)

const (
	statusSuccess          = "success"
	statusNoClassification = "no_classification"
)

// ClassifyResult is the MCP-facing classification response payload.
type ClassifyResult struct {
	Status     string                `json:"status"`
	Sections   []ClassifiedSection   `json:"sections"`
	SlicingMap map[string]AgentSlice `json:"slicing_map"`
	Error      string                `json:"error,omitempty"`
}

// ClassifiedSection includes original section metadata and assignments.
type ClassifiedSection struct {
	SectionID   int                 `json:"section_id"`
	Heading     string              `json:"heading"`
	LineCount   int                 `json:"line_count"`
	Assignments []SectionAssignment `json:"assignments"`
}

// SectionAssignment maps a section to an agent with relevance weight.
type SectionAssignment struct {
	Agent      string  `json:"agent"`
	Relevance  string  `json:"relevance"`
	Confidence float64 `json:"confidence"`
}

// AgentSlice summarizes which sections each agent should read first vs context.
type AgentSlice struct {
	PrioritySections   []int `json:"priority_sections"`
	ContextSections    []int `json:"context_sections"`
	TotalPriorityLines int   `json:"total_priority_lines"`
	TotalContextLines  int   `json:"total_context_lines"`
}

type dispatchResponse struct {
	Sections []dispatchSection `json:"sections"`
}

type dispatchSection struct {
	SectionID   int                 `json:"section_id"`
	Assignments []SectionAssignment `json:"assignments"`
}

// Classify runs Codex spark dispatch and produces section slicing metadata.
func Classify(ctx context.Context, dispatchPath string, sections []extract.Section, agents []AgentDomain) ClassifyResult {
	if len(agents) == 0 {
		agents = DefaultAgents()
	}
	if len(sections) == 0 {
		return ClassifyResult{
			Status:     statusNoClassification,
			Sections:   []ClassifiedSection{},
			SlicingMap: map[string]AgentSlice{},
			Error:      "no sections to classify",
		}
	}

	prompt := BuildPrompt(sections, agents)

	promptFile, err := os.CreateTemp("", "clodex-prompt-*.txt")
	if err != nil {
		return classifyError(err, sections, agents, "create prompt temp file")
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(prompt); err != nil {
		_ = promptFile.Close()
		return classifyError(err, sections, agents, "write prompt temp file")
	}
	if err := promptFile.Close(); err != nil {
		return classifyError(err, sections, agents, "close prompt temp file")
	}

	outputFile, err := os.CreateTemp("", "clodex-output-*.json")
	if err != nil {
		return classifyError(err, sections, agents, "create output temp file")
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return classifyError(err, sections, agents, "close output temp file")
	}
	defer os.Remove(outputPath)

	cmd := exec.CommandContext(
		ctx,
		"bash",
		dispatchPath,
		"--tier", "fast",
		"--sandbox", "read-only",
		"--prompt-file", promptPath,
		"-o", outputPath,
	)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		stderr := strings.TrimSpace(string(combined))
		if stderr == "" {
			stderr = err.Error()
		}
		return ClassifyResult{
			Status:     statusNoClassification,
			Sections:   buildEmptySections(sections),
			SlicingMap: buildEmptySlicingMap(agents),
			Error:      fmt.Sprintf("dispatch failed: %s", stderr),
		}
	}

	rawOutput, err := os.ReadFile(outputPath)
	if err != nil {
		return classifyError(err, sections, agents, "read dispatch output")
	}

	payload := strings.TrimSpace(string(rawOutput))
	if payload == "" {
		payload = strings.TrimSpace(string(combined))
	}
	payload = stripCodeFences(payload)
	if payload == "" {
		return ClassifyResult{
			Status:     statusNoClassification,
			Sections:   buildEmptySections(sections),
			SlicingMap: buildEmptySlicingMap(agents),
			Error:      "dispatch returned empty classification output",
		}
	}

	var decoded dispatchResponse
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return ClassifyResult{
			Status:     statusNoClassification,
			Sections:   buildEmptySections(sections),
			SlicingMap: buildEmptySlicingMap(agents),
			Error:      fmt.Sprintf("invalid classification JSON: %v", err),
		}
	}

	classified := make(map[int][]SectionAssignment, len(decoded.Sections))
	for _, section := range decoded.Sections {
		classified[section.SectionID] = append(classified[section.SectionID], section.Assignments...)
	}

	return buildResult(classified, sections, agents)
}

func classifyError(err error, sections []extract.Section, agents []AgentDomain, context string) ClassifyResult {
	return ClassifyResult{
		Status:     statusNoClassification,
		Sections:   buildEmptySections(sections),
		SlicingMap: buildEmptySlicingMap(agents),
		Error:      fmt.Sprintf("%s: %v", context, err),
	}
}

func buildEmptySections(sections []extract.Section) []ClassifiedSection {
	out := make([]ClassifiedSection, 0, len(sections))
	for _, section := range sections {
		out = append(out, ClassifiedSection{
			SectionID:   section.ID,
			Heading:     section.Heading,
			LineCount:   section.LineCount,
			Assignments: []SectionAssignment{},
		})
	}
	return out
}

func buildEmptySlicingMap(agents []AgentDomain) map[string]AgentSlice {
	if len(agents) == 0 {
		agents = DefaultAgents()
	}
	out := make(map[string]AgentSlice, len(agents))
	for _, agent := range agents {
		out[agent.Name] = AgentSlice{
			PrioritySections: []int{},
			ContextSections:  []int{},
		}
	}
	return out
}

func buildResult(classified map[int][]SectionAssignment, sections []extract.Section, agents []AgentDomain) ClassifyResult {
	if len(agents) == 0 {
		agents = DefaultAgents()
	}

	allowed := make(map[string]bool, len(agents)+len(CrossCuttingAgents))
	for _, agent := range agents {
		allowed[agent.Name] = true
	}
	for agent := range CrossCuttingAgents {
		allowed[agent] = true
	}

	result := ClassifyResult{
		Status:     statusNoClassification,
		Sections:   make([]ClassifiedSection, 0, len(sections)),
		SlicingMap: buildEmptySlicingMap(agents),
	}

	prioritySeen := make(map[string]map[int]bool)
	contextSeen := make(map[string]map[int]bool)

	totalLines := 0
	for _, section := range sections {
		totalLines += section.LineCount
		normalized := normalizeAssignments(classified[section.ID], allowed)
		result.Sections = append(result.Sections, ClassifiedSection{
			SectionID:   section.ID,
			Heading:     section.Heading,
			LineCount:   section.LineCount,
			Assignments: normalized,
		})

		for _, assignment := range normalized {
			slice := result.SlicingMap[assignment.Agent]
			if assignment.Relevance == "priority" {
				if prioritySeen[assignment.Agent] == nil {
					prioritySeen[assignment.Agent] = map[int]bool{}
				}
				if !prioritySeen[assignment.Agent][section.ID] {
					slice.PrioritySections = append(slice.PrioritySections, section.ID)
					slice.TotalPriorityLines += section.LineCount
					prioritySeen[assignment.Agent][section.ID] = true
				}
			} else {
				if contextSeen[assignment.Agent] == nil {
					contextSeen[assignment.Agent] = map[int]bool{}
				}
				if !contextSeen[assignment.Agent][section.ID] {
					slice.ContextSections = append(slice.ContextSections, section.ID)
					slice.TotalContextLines += section.LineCount
					contextSeen[assignment.Agent][section.ID] = true
				}
			}
			result.SlicingMap[assignment.Agent] = slice
		}
	}

	for agent, slice := range result.SlicingMap {
		sort.Ints(slice.PrioritySections)
		sort.Ints(slice.ContextSections)
		result.SlicingMap[agent] = slice
	}

	if totalLines <= 0 {
		return result
	}

	// Domain mismatch guard: if no agent has >10% priority lines, classification likely failed.
	anyAboveThreshold := false
	for _, agent := range agents {
		if result.SlicingMap[agent.Name].TotalPriorityLines*100/totalLines > 10 {
			anyAboveThreshold = true
			break
		}
	}
	if !anyAboveThreshold {
		result.Error = "domain mismatch: no agent has >10% priority lines"
		return result
	}

	// Classification succeeded — apply per-agent 80% threshold.
	result.Status = statusSuccess
	allSectionIDs := make([]int, 0, len(sections))
	for _, s := range sections {
		allSectionIDs = append(allSectionIDs, s.ID)
	}
	for _, agent := range agents {
		slice := result.SlicingMap[agent.Name]
		// Integer arithmetic: priority_lines*100/total_lines >= 80 → send full doc.
		if slice.TotalPriorityLines*100/totalLines >= 80 {
			slice.PrioritySections = allSectionIDs
			slice.TotalPriorityLines = totalLines
			slice.ContextSections = nil
			slice.TotalContextLines = 0
			result.SlicingMap[agent.Name] = slice
		}
	}

	return result
}

func normalizeAssignments(in []SectionAssignment, allowed map[string]bool) []SectionAssignment {
	out := make([]SectionAssignment, 0, len(in))
	for _, a := range in {
		a.Agent = strings.TrimSpace(a.Agent)
		a.Relevance = strings.TrimSpace(strings.ToLower(a.Relevance))
		if !allowed[a.Agent] {
			continue
		}
		if a.Relevance != "priority" && a.Relevance != "context" {
			continue
		}
		if a.Confidence < 0 {
			a.Confidence = 0
		}
		if a.Confidence > 1 {
			a.Confidence = 1
		}
		out = append(out, a)
	}
	return out
}

func stripCodeFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return ""
	}

	lines = lines[1:]
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

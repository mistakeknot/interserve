package extract

import (
	"fmt"
	"strings"
)

// Section is a markdown slice rooted at a top-level (##) heading.
type Section struct {
	ID        int
	Heading   string
	Body      string
	LineCount int
}

// ExtractSections splits a markdown document into sections by "## " headings.
// It ignores headings inside fenced code blocks and skips YAML frontmatter.
func ExtractSections(doc string) []Section {
	lines := splitLines(doc)
	lines = skipYAMLFrontmatter(lines)

	sections := make([]Section, 0)
	nextID := 1

	currentHeading := "Preamble"
	currentBody := make([]string, 0)
	hasSeenHeading := false
	inFence := false
	fence := ""

	emit := func(heading string, bodyLines []string, isPreamble bool) {
		body := strings.Join(bodyLines, "\n")
		if isPreamble && strings.TrimSpace(body) == "" {
			return
		}
		sections = append(sections, Section{
			ID:        nextID,
			Heading:   heading,
			Body:      body,
			LineCount: len(bodyLines),
		})
		nextID++
	}

	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " \t")

		if !inFence && strings.HasPrefix(trimmedLeft, "## ") {
			emit(currentHeading, currentBody, !hasSeenHeading)
			hasSeenHeading = true
			currentHeading = strings.TrimSpace(strings.TrimPrefix(trimmedLeft, "## "))
			currentBody = make([]string, 0)
			continue
		}

		currentBody = append(currentBody, line)

		if marker := fenceMarker(trimmedLeft); marker != "" {
			if !inFence {
				inFence = true
				fence = marker
			} else if marker == fence {
				inFence = false
				fence = ""
			}
		}
	}

	emit(currentHeading, currentBody, !hasSeenHeading)
	return sections
}

// Preview returns an adaptive section preview.
func (s Section) Preview() string {
	lines := splitBodyLines(s.Body)
	count := len(lines)
	if count == 0 {
		return ""
	}

	if count <= 100 {
		end := min(50, count)
		return strings.Join(lines[:end], "\n")
	}

	omitted := count - 50
	head := strings.Join(lines[:25], "\n")
	tail := strings.Join(lines[count-25:], "\n")
	return fmt.Sprintf("%s\n[... %d lines omitted ...]\n%s", head, omitted, tail)
}

// FirstSentence returns the first non-empty, non-fence line (max 120 chars).
func (s Section) FirstSentence() string {
	for _, line := range splitBodyLines(s.Body) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if marker := fenceMarker(trimmed); marker != "" {
			continue
		}
		return truncateRunes(trimmed, 120)
	}
	return ""
}

func splitLines(doc string) []string {
	if doc == "" {
		return []string{}
	}
	return strings.Split(doc, "\n")
}

func skipYAMLFrontmatter(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return lines
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[i+1:]
		}
	}

	// Unclosed frontmatter: treat it as metadata and skip the full document body.
	return []string{}
}

func splitBodyLines(body string) []string {
	if body == "" {
		return []string{}
	}
	return strings.Split(body, "\n")
}

func fenceMarker(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		return "```"
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~"
	}
	return ""
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}


package extract

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractSectionsBasic(t *testing.T) {
	doc := "Intro text\n## A\nalpha\n## B\nbeta"

	sections := ExtractSections(doc)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].Heading != "Preamble" {
		t.Fatalf("expected preamble heading, got %q", sections[0].Heading)
	}
	if sections[1].Heading != "A" || sections[2].Heading != "B" {
		t.Fatalf("unexpected headings: %q, %q", sections[1].Heading, sections[2].Heading)
	}
}

func TestExtractSectionsCodeBlockIgnoresHashes(t *testing.T) {
	doc := "## A\n```go\n## inside code\n```\noutside\n## B\nbody"

	sections := ExtractSections(doc)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if !strings.Contains(sections[0].Body, "## inside code") {
		t.Fatalf("expected fenced heading-like text to remain in section body")
	}
}

func TestExtractSectionsTildeCodeBlockIgnoresHashes(t *testing.T) {
	doc := "## A\n~~~txt\n## still code\n~~~\n## B\nend"

	sections := ExtractSections(doc)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if !strings.Contains(sections[0].Body, "## still code") {
		t.Fatalf("expected tilde fenced heading-like text to remain in section body")
	}
}

func TestExtractSectionsUnclosedCodeBlock(t *testing.T) {
	doc := "## A\n```markdown\n## not a heading\nstill code\n## also not heading"

	sections := ExtractSections(doc)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section due to unclosed fence, got %d", len(sections))
	}
	if sections[0].Heading != "A" {
		t.Fatalf("unexpected heading %q", sections[0].Heading)
	}
}

func TestExtractSectionsSkipsYAMLFrontmatter(t *testing.T) {
	doc := "---\ntitle: Example\nowner: team\n---\n\n## A\nbody"

	sections := ExtractSections(doc)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section after frontmatter removal, got %d", len(sections))
	}
	if sections[0].Heading != "A" {
		t.Fatalf("expected heading A, got %q", sections[0].Heading)
	}
}

func TestExtractSectionsKeepsEmptySections(t *testing.T) {
	doc := "## A\n## B\nbody"

	sections := ExtractSections(doc)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].LineCount != 0 {
		t.Fatalf("expected first section line count 0, got %d", sections[0].LineCount)
	}
}

func TestPreviewSmallSection(t *testing.T) {
	section := Section{Body: joinNumberedLines("small", 60)}
	preview := section.Preview()
	lines := strings.Split(preview, "\n")

	if len(lines) != 50 {
		t.Fatalf("expected 50 lines in preview, got %d", len(lines))
	}
	if strings.Contains(preview, "lines omitted") {
		t.Fatalf("did not expect omission marker for <=100-line section")
	}
}

func TestPreviewLargeSection(t *testing.T) {
	section := Section{Body: joinNumberedLines("large", 120)}
	preview := section.Preview()
	lines := strings.Split(preview, "\n")

	if len(lines) != 51 {
		t.Fatalf("expected 51 lines (25 + marker + 25), got %d", len(lines))
	}
	if !strings.Contains(preview, "[... 70 lines omitted ...]") {
		t.Fatalf("missing or incorrect omission marker")
	}
}

func joinNumberedLines(prefix string, n int) string {
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		lines[i] = fmt.Sprintf("%s-%03d", prefix, i+1)
	}
	return strings.Join(lines, "\n")
}

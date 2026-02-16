package query

import (
	"fmt"
	"strings"
)

const (
	ModeAnswer    = "answer"
	ModeSummarize = "summarize"
	ModeExtract   = "extract"

	maxFileLines     = 10000
	headLines        = 5000
	tailLines        = 2000
	maxFileSizeBytes = 1 << 20 // 1 MB
)

// BuildPrompt constructs a mode-specific prompt for Codex file analysis.
func BuildPrompt(question string, files map[string]string, mode string) string {
	var b strings.Builder

	b.WriteString("Be EXTREMELY concise. 10-20 lines max. No preamble. No repeating the question.\n\n")

	switch mode {
	case ModeSummarize:
		b.WriteString("Provide a structural overview of the file(s):\n")
		b.WriteString("- List key types, functions, and their purposes (one line each)\n")
		b.WriteString("- Note important constants, interfaces, and exported symbols\n")
		b.WriteString("- Identify the main responsibility/pattern of each file\n")
		b.WriteString("- Skip imports, boilerplate, and obvious details\n\n")
	case ModeExtract:
		if question != "" {
			fmt.Fprintf(&b, "Extract the specific code snippets relevant to: %s\n", question)
			b.WriteString("- Include only the directly relevant lines with path:line_number prefixes\n")
			b.WriteString("- Add minimal context (1-2 lines) around each snippet\n")
			b.WriteString("- Omit everything else\n\n")
		}
	default: // ModeAnswer
		if question != "" {
			fmt.Fprintf(&b, "Question: %s\n\n", question)
		}
		b.WriteString("Answer based on the file content below. Cite specific lines as path:N.\n\n")
	}

	for path, content := range files {
		lines := strings.Split(content, "\n")
		totalLines := len(lines)

		fmt.Fprintf(&b, "--- %s (%d lines) ---\n", path, totalLines)

		if totalLines > maxFileLines {
			// Head + tail with omission marker
			for i, line := range lines[:headLines] {
				fmt.Fprintf(&b, "%s:%d\t%s\n", path, i+1, line)
			}
			fmt.Fprintf(&b, "\n[... %d lines omitted ...]\n\n", totalLines-headLines-tailLines)
			for i := totalLines - tailLines; i < totalLines; i++ {
				fmt.Fprintf(&b, "%s:%d\t%s\n", path, i+1, lines[i])
			}
		} else {
			for i, line := range lines {
				fmt.Fprintf(&b, "%s:%d\t%s\n", path, i+1, line)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

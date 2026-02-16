package query

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Prompt tests ---

func TestBuildAnswerPrompt(t *testing.T) {
	files := map[string]string{
		"/tmp/test.go": "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
	}
	prompt := BuildPrompt("What does main do?", files, ModeAnswer)

	if !strings.Contains(prompt, "What does main do?") {
		t.Fatal("answer prompt missing question")
	}
	if !strings.Contains(prompt, "/tmp/test.go:1") {
		t.Fatal("answer prompt missing line-numbered content")
	}
	if !strings.Contains(prompt, "Be EXTREMELY concise") {
		t.Fatal("answer prompt missing conciseness instruction")
	}
}

func TestBuildSummarizePrompt(t *testing.T) {
	files := map[string]string{
		"/tmp/test.go": "package main\n\ntype Foo struct{}\n",
	}
	prompt := BuildPrompt("", files, ModeSummarize)

	if !strings.Contains(prompt, "structural overview") {
		t.Fatal("summarize prompt missing structural extraction instruction")
	}
	if !strings.Contains(prompt, "key types, functions") {
		t.Fatal("summarize prompt missing types/functions instruction")
	}
}

func TestBuildExtractPrompt(t *testing.T) {
	files := map[string]string{
		"/tmp/test.go": "package main\n\nfunc Foo() {}\nfunc Bar() {}\n",
	}
	prompt := BuildPrompt("Find the Foo function", files, ModeExtract)

	if !strings.Contains(prompt, "Extract the specific code snippets") {
		t.Fatal("extract prompt missing extraction instruction")
	}
	if !strings.Contains(prompt, "Find the Foo function") {
		t.Fatal("extract prompt missing question")
	}
}

func TestBuildPromptMultipleFiles(t *testing.T) {
	files := map[string]string{
		"/tmp/a.go": "package a\n",
		"/tmp/b.go": "package b\n",
	}
	prompt := BuildPrompt("question", files, ModeAnswer)

	if !strings.Contains(prompt, "--- /tmp/a.go") {
		t.Fatal("multi-file prompt missing first file separator")
	}
	if !strings.Contains(prompt, "--- /tmp/b.go") {
		t.Fatal("multi-file prompt missing second file separator")
	}
}

func TestBuildPromptTruncatesLargeFiles(t *testing.T) {
	lines := make([]string, maxFileLines+500)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	content := strings.Join(lines, "\n")

	files := map[string]string{"/tmp/big.go": content}
	prompt := BuildPrompt("question", files, ModeAnswer)

	if !strings.Contains(prompt, "[...") {
		t.Fatal("large file prompt missing omission marker")
	}
	// Should contain head lines
	if !strings.Contains(prompt, "/tmp/big.go:1\t") {
		t.Fatal("large file prompt missing head content")
	}
	// Should contain tail lines
	lastLine := fmt.Sprintf("/tmp/big.go:%d\t", maxFileLines+500)
	if !strings.Contains(prompt, lastLine) {
		t.Fatal("large file prompt missing tail content")
	}
	// Should NOT contain lines in the omitted range
	omittedLine := fmt.Sprintf("/tmp/big.go:%d\t", headLines+100)
	if strings.Contains(prompt, omittedLine) {
		t.Fatal("large file prompt contains lines that should be omitted")
	}
}

// --- Query dispatch tests ---
// These test input validation and response parsing without requiring Codex.

func TestQueryErrorOnMissingFile(t *testing.T) {
	result := Query(context.Background(), "/nonexistent/dispatch.sh", "question", []string{"/nonexistent/file.go"}, ModeAnswer)
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if !strings.Contains(result.Error, "file not found") {
		t.Fatalf("expected file not found error, got %q", result.Error)
	}
}

func TestQueryErrorOnEmptyQuestion(t *testing.T) {
	result := Query(context.Background(), "/nonexistent/dispatch.sh", "", []string{"/tmp/test.go"}, ModeAnswer)
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if !strings.Contains(result.Error, "question is required") {
		t.Fatalf("expected question required error, got %q", result.Error)
	}
}

func TestQuerySummarizeModeAllowsEmptyQuestion(t *testing.T) {
	// Create a real temp file so validation passes — dispatch will fail but that's fine
	tmp := writeTempFile(t, "package main\n")
	defer os.Remove(tmp)

	result := Query(context.Background(), "/nonexistent/dispatch.sh", "", []string{tmp}, ModeSummarize)
	// Should get past validation (dispatch will fail since path doesn't exist)
	if result.Error == "question is required for answer mode" {
		t.Fatal("summarize mode should not require a question")
	}
}

func TestQueryMaxFileSize(t *testing.T) {
	// Create a file larger than 1MB
	tmp := filepath.Join(t.TempDir(), "large.go")
	data := make([]byte, maxFileSizeBytes+1)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatal(err)
	}

	result := Query(context.Background(), "/nonexistent/dispatch.sh", "question", []string{tmp}, ModeAnswer)
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if !strings.Contains(result.Error, "file too large") {
		t.Fatalf("expected file too large error, got %q", result.Error)
	}
}

func TestQueryInvalidMode(t *testing.T) {
	result := Query(context.Background(), "/nonexistent/dispatch.sh", "question", []string{"/tmp/test.go"}, "invalid")
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if !strings.Contains(result.Error, "invalid mode") {
		t.Fatalf("expected invalid mode error, got %q", result.Error)
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no fences", "hello world", "hello world"},
		{"json fences", "```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"plain fences", "```\nsome text\n```", "some text"},
		{"trailing blank lines", "```\ntext\n\n```", "text"},
		{"no closing fence", "```\ntext only", "text only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.expected {
				t.Fatalf("stripCodeFences(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestQueryNoFiles(t *testing.T) {
	result := Query(context.Background(), "/nonexistent/dispatch.sh", "question", nil, ModeAnswer)
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if !strings.Contains(result.Error, "at least one file") {
		t.Fatalf("expected no files error, got %q", result.Error)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "interserve-test-*.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

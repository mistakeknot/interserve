package query

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// QueryResult is the MCP-facing response payload for codex_query.
type QueryResult struct {
	Status         string   `json:"status"`
	Answer         string   `json:"answer"`
	FilesAnalyzed  []string `json:"files_analyzed"`
	LineCountSaved int      `json:"line_count_saved"`
	Mode           string   `json:"mode"`
	Error          string   `json:"error,omitempty"`
}

// Query reads the given files, sends them to Codex via dispatch.sh, and returns a compact answer.
func Query(ctx context.Context, dispatchPath string, question string, files []string, mode string) QueryResult {
	if mode == "" {
		mode = ModeAnswer
	}
	if mode != ModeAnswer && mode != ModeSummarize && mode != ModeExtract {
		return QueryResult{
			Status: "error",
			Mode:   mode,
			Error:  fmt.Sprintf("invalid mode %q: must be answer, summarize, or extract", mode),
		}
	}
	if mode == ModeAnswer && strings.TrimSpace(question) == "" {
		return QueryResult{
			Status: "error",
			Mode:   mode,
			Error:  "question is required for answer mode",
		}
	}
	if len(files) == 0 {
		return QueryResult{
			Status: "error",
			Mode:   mode,
			Error:  "at least one file is required",
		}
	}

	// Read files into memory, validate existence and size.
	fileContents := make(map[string]string, len(files))
	totalLines := 0
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return QueryResult{
				Status:        "error",
				Mode:          mode,
				FilesAnalyzed: files,
				Error:         fmt.Sprintf("file not found: %s", path),
			}
		}
		if info.Size() > maxFileSizeBytes {
			return QueryResult{
				Status:        "error",
				Mode:          mode,
				FilesAnalyzed: files,
				Error:         fmt.Sprintf("file too large (%d bytes, max %d): %s", info.Size(), maxFileSizeBytes, path),
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return QueryResult{
				Status:        "error",
				Mode:          mode,
				FilesAnalyzed: files,
				Error:         fmt.Sprintf("read %s: %v", path, err),
			}
		}
		content := string(data)
		fileContents[path] = content
		totalLines += len(strings.Split(content, "\n"))
	}

	// Build prompt and dispatch to Codex.
	prompt := BuildPrompt(question, fileContents, mode)

	promptFile, err := os.CreateTemp("", "interserve-query-prompt-*.txt")
	if err != nil {
		return queryError(err, files, mode, "create prompt temp file")
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(prompt); err != nil {
		_ = promptFile.Close()
		return queryError(err, files, mode, "write prompt temp file")
	}
	if err := promptFile.Close(); err != nil {
		return queryError(err, files, mode, "close prompt temp file")
	}

	outputFile, err := os.CreateTemp("", "interserve-query-output-*.txt")
	if err != nil {
		return queryError(err, files, mode, "create output temp file")
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return queryError(err, files, mode, "close output temp file")
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
		return QueryResult{
			Status:        "error",
			Mode:          mode,
			FilesAnalyzed: files,
			Error:         fmt.Sprintf("dispatch failed: %s", stderr),
		}
	}

	rawOutput, err := os.ReadFile(outputPath)
	if err != nil {
		return queryError(err, files, mode, "read dispatch output")
	}

	answer := strings.TrimSpace(string(rawOutput))
	if answer == "" {
		answer = strings.TrimSpace(string(combined))
	}
	answer = stripCodeFences(answer)

	if answer == "" {
		return QueryResult{
			Status:        "error",
			Mode:          mode,
			FilesAnalyzed: files,
			Error:         "dispatch returned empty output",
		}
	}

	return QueryResult{
		Status:         "success",
		Answer:         answer,
		FilesAnalyzed:  files,
		LineCountSaved: totalLines,
		Mode:           mode,
	}
}

func queryError(err error, files []string, mode string, context string) QueryResult {
	return QueryResult{
		Status:        "error",
		Mode:          mode,
		FilesAnalyzed: files,
		Error:         fmt.Sprintf("%s: %v", context, err),
	}
}

// stripCodeFences removes leading ```<lang> and trailing ``` from LLM output.
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
